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

func newRandReader(rd io.Reader) io.Reader {
	return &randReader{rd: rd, buf: make([]byte, 0, 7)}
}

func (rd *randReader) Read(p []byte) (int, error) {
	// first, copy buffer to p
	pos := copy(p, rd.buf)

	// if this is enough to fill p, return
	if pos == len(p) {
		return pos, nil
	}

	// else p is larger than buf, set length to zero
	rd.buf = rd.buf[:0]

	// load multiple of 7 byte to temp buffer
	bufsize := ((len(p) / 7) + 1) * 7
	buf := make([]byte, bufsize)
	n, err := io.ReadFull(rd.rd, buf)
	if err != nil {
		return pos, err
	}

	// copy the buffer to p
	n = copy(p[pos:], buf)
	pos += n

	// save the remaining bytes in rd.buf
	rd.buf = rd.buf[:7]
	n = copy(rd.buf, buf[len(p):])
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

	fmt.Printf("\nnew file, seed 0x%x, size %v\n", seed, size)

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

		fmt.Printf("  generate segment %v, %v\n", segmentIndex, segment)

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
	fmt.Printf("ReadAt(len %v, off %v)\n", len(p), off)
	if off < 0 {
		return 0, errors.New("not implemented")
	}

	if off > int64(f.Size) {
		return 0, errors.New("offset beyond end of file")
	}

	pos := 0
	for i, seg := range f.Segments {
		if off > int64(seg.Size) {
			fmt.Printf("  skip segment %v, off %v, size %v\n", i, off, seg.Size)
			off -= int64(seg.Size)
			continue
		}

		fmt.Printf("   found segment %v, off %v, size %v\n", i, off, seg.Size)

		maxRead := pos + seg.Size - int(off)
		if maxRead > len(p) {
			maxRead = len(p)
		}

		rd := seg.Reader()
		if off > 0 {
			fmt.Printf("    discard %d bytes\n", off)
			_, err = io.CopyN(ioutil.Discard, rd, off)
			if err != nil {
				return 0, err
			}

			off = 0
		}

		fmt.Printf("     read to p[%d:%d], len(p) %v\n", pos, maxRead, len(p))
		n, err := io.ReadFull(rd, p[pos:maxRead])
		fmt.Printf("      -> n %v, err %v\n", n, err)
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

// ReadAll returns the content of the file.
func (f FileHandle) ReadAll(ctx context.Context) ([]byte, error) {
	return f.f.ReadAll()
}