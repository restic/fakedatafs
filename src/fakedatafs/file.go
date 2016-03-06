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
	copy(rd.buf, rd.buf[pos:])

	// shorten buf and p accordingly
	rd.buf = rd.buf[:len(rd.buf)-pos]
	p = p[pos:]

	// if this is enough to fill p, return
	if len(p) == 0 {
		return pos, nil
	}

	// else p is larger than buf

	// load multiple of 7 byte to temp buffer
	bufsize := ((len(p) / 7) + 1) * 7
	buf := make([]byte, bufsize)
	n, err := io.ReadFull(rd.rd, buf)
	if err != nil {
		return pos, err
	}

	// copy the buffer to p
	n = copy(p, buf)
	pos += n

	// save the remaining bytes in rd.buf
	rd.buf = rd.buf[:7]
	n = copy(rd.buf, buf[len(p):])
	rd.buf = rd.buf[:n]

	return pos, nil
}

type dumpReader struct {
	rd io.Reader
}

func (d dumpReader) Read(p []byte) (int, error) {
	n, err := d.rd.Read(p)
	max := 20
	if n < max {
		max = n
	}
	return n, err
}

// Reader returns a reader for this segment.
func (s Segment) Reader() io.Reader {
	rd := dumpReader{rd: rand.New(rand.NewSource(s.Seed))}
	return newRandReader(rd)
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
