package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"

	"github.com/jacobsa/fuse/fuseops"
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
	return io.LimitReader(newRandReader(rand.New(rand.NewSource(s.Seed))), int64(s.Size))
}

// File represents fake data with a specific seed.
type File struct {
	Seed     int64
	Size     int
	Inode    fuseops.InodeID
	Segments []Segment

	pos int64
}

func (f File) String() string {
	return fmt.Sprintf("<File seed 0x%x, Size %d>", f.Seed, f.Size)
}

// NewFile initializes a new file with the given seed.
func NewFile(seed int64, size int, inode fuseops.InodeID) *File {
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

type contFileReader struct {
	Segments []Segment
	seg      int
	size     int
	skip     int64
	cur      io.Reader
}

// ContinuousFileReader returns a reader that yields the content of a file
// starting at start.
func ContinuousFileReader(f *File, start int64) io.Reader {
	return &contFileReader{Segments: f.Segments, skip: start, size: f.Size}
}

func (rd *contFileReader) Read(p []byte) (int, error) {

	if rd.skip > int64(rd.size) {
		return 0, errors.New("offset beyond end of file")
	}

	// skip whole segments
	if rd.skip > 0 {
		for i, s := range rd.Segments {

			if rd.skip < int64(s.Size) {
				rd.seg = i
				break
			}

			rd.skip -= int64(s.Size)
		}
	}

	// skip bytes of current reader
	if rd.cur == nil {
		seg := rd.Segments[rd.seg]
		rd.cur = seg.Reader()

		if rd.skip > 0 {
			_, err := io.CopyN(ioutil.Discard, rd.cur, rd.skip)
			if err != nil {
				return 0, err
			}

			rd.skip = 0
		}
	}

	// read data
	pos := 0
	for pos < len(p) {
		n, err := io.ReadFull(rd.cur, p[pos:])
		pos += n

		if err == io.ErrUnexpectedEOF {
			rd.seg++
			if rd.seg >= len(rd.Segments) {
				rd.cur = nil
				return pos, io.EOF
			}

			rd.cur = rd.Segments[rd.seg].Reader()
			continue
		}

		if err != nil {
			return n, err
		}
	}

	return pos, nil
}
