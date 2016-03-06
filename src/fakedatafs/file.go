package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

const minSegmentSize = 512 * 1024      // 512KiB
const maxSegmentSize = 4 * 1024 * 1024 // 4MiB

// Segment is one segment within a file.
type Segment struct {
	Seed int64
	Size int
}

func (s Segment) String() string {
	return fmt.Sprintf("<Segment seed 0x%x, len %v>", s.Seed, s.Size)
}

type randReader struct {
	rd  io.Reader
	buf []byte
}

type dumpReader struct {
	rd io.Reader
}

func (d dumpReader) Read(p []byte) (int, error) {
	n, err := d.rd.Read(p)
	fmt.Printf("dump: Read(%d) = %v, %v\n", len(p), n, err)
	return n, err
}

func newRandReader(rd io.Reader) io.Reader {
	return &randReader{rd: rd, buf: make([]byte, 0, 7)}
}

func (rd *randReader) Read(p []byte) (int, error) {
	// first, copy buffer to p
	pos := copy(p, rd.buf)
	copy(rd.buf, rd.buf[pos:])

	// shorten buf and p accordingly
	rd.buf = rd.buf[:len(rd.buf)-pos]
	p = p[pos:]

	// if this is enough to fill p, return
	if len(p) == 0 {
		return pos, nil
	}

	// load multiple of 7 byte
	l := (len(p) / 7) * 7
	n, err := io.ReadFull(rd.rd, p[:l])
	pos += n
	if err != nil {
		return pos, err
	}
	p = p[n:]

	// load 7 byte to temp buffer
	rd.buf = rd.buf[:7]
	n, err = io.ReadFull(rd.rd, rd.buf)
	if err != nil {
		return pos, err
	}

	// copy the remaining bytes from the buffer to p
	n = copy(p, rd.buf)
	pos += n

	// save the remaining bytes in rd.buf
	n = copy(rd.buf, rd.buf[n:])
	rd.buf = rd.buf[:n]

	return pos, nil
}

// Reader returns a reader for this segment.
func (s Segment) Reader() io.Reader {
	return newRandReader(rand.New(rand.NewSource(s.Seed)))
}

// File represents fake data with a specific seed.
type File struct {
	Seed     int64
	Size     int
	Inode    uint64
	Segments []Segment

	pos int64
}

func (f File) String() string {
	return fmt.Sprintf("<File seed 0x%x, Size %d>", f.Seed, f.Size)
}

// NewFile initializes a new file with the given seed.
func NewFile(seed int64, size int, inode uint64) *File {
	f := &File{
		Seed:  seed,
		Inode: inode,
	}

	src := rand.New(rand.NewSource(seed))
	var segmentIndex int64
	for f.Size < size {
		nextSize := size - f.Size
		if nextSize > minSegmentSize {
			max := nextSize - minSegmentSize
			if max > maxSegmentSize {
				max = maxSegmentSize
			}
			nextSize = src.Intn(max) + minSegmentSize
		}

		segment := Segment{
			Seed: seed ^ segmentIndex,
			Size: nextSize,
		}
		f.Segments = append(f.Segments, segment)

		f.Size += nextSize
		segmentIndex++
	}

	return f
}

// ReadAll returns the content of the file.
func (f File) ReadAll() ([]byte, error) {
	buf := make([]byte, f.Size)
	pos := 0
	for _, seg := range f.Segments {
		n, err := io.ReadFull(seg.Reader(), buf[pos:pos+seg.Size])
		pos += n
		if err != nil {
			return buf[:pos], err
		}
	}

	if pos != f.Size {
		return buf, io.ErrUnexpectedEOF
	}

	return buf, nil
}

// ReadAt reads the content at the offset.
func (f File) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("not implemented")
	}

	if off > int64(f.Size) {
		return 0, errors.New("offset beyond end of file")
	}

	pos := 0
	for _, seg := range f.Segments {
		if off > int64(seg.Size) {
			off -= int64(seg.Size)
			continue
		}

		maxRead := pos + seg.Size - int(off)
		if maxRead > len(p) {
			maxRead = len(p)
		}

		rd := seg.Reader()
		if off > 0 {
			_, err = io.CopyN(ioutil.Discard, rd, off)
			if err != nil {
				return 0, err
			}

			off = 0
		}

		n, err := io.ReadFull(rd, p[pos:maxRead])
		pos += n
		if err != nil {
			return pos, err
		}

		if pos == len(p) {
			break
		}
	}

	return pos, nil
}

// Read reads data into a buffer
func (f *File) Read(p []byte) (n int, err error) {
	n, err = f.ReadAt(p, f.pos)
	f.pos += int64(n)
	return n, err
}

// Seek to the given position.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	pos := f.pos
	switch whence {
	case 0:
		pos = offset
	case 1:
		pos += offset
	case 2:
		pos = int64(f.Size) - offset
	}
	if pos < 0 {
		return 0, errors.New("invalid negative position in file")
	}

	f.pos = pos

	return pos, nil
}

// Attr returns the attributes for this file.
func (f File) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = f.Inode
	a.Mode = 0644
	a.Size = uint64(f.Size)
	return nil
}

// Open the file.
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	return FileHandle{f}, nil
}

// FileHandle is return by Open.
type FileHandle struct {
	f *File
}

// Read returns the content of the file.
func (f FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, res *fuse.ReadResponse) error {
	// fmt.Printf("Read %v byte at %v, res.Data len %v, cap %v\n", req.Size, req.Offset, len(res.Data), cap(res.Data))
	res.Data = res.Data[:req.Size]
	n, err := f.f.ReadAt(res.Data, req.Offset)
	// fmt.Printf("  -> %v %v\n", n, err)
	res.Data = res.Data[:n]
	return err
}

// ReadAll returns the content of the file.
// func (f FileHandle) ReadAll(ctx context.Context) ([]byte, error) {
// 	return f.f.ReadAll()
// }
