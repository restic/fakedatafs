package main

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
)

var testFileSizes = []int{0, 100, 200, 500, 1024, 7666, 1 << 20, 1 << 24, 1<<22 + 1234}

func TestFile(t *testing.T) {
	rnd := rand.New(rand.NewSource(23))

	for i, filesize := range testFileSizes {
		f := NewFile(23, filesize, 23)
		buf, err := f.ReadAll()
		if err != nil {
			t.Errorf("test %d: error %v", i, err)
			continue
		}

		if len(buf) != filesize {
			t.Errorf("test %d: invalid number of bytes returned, want %d, got %d", i, filesize, len(buf))
			continue
		}

		buf2 := make([]byte, filesize)
		n, err := f.ReadAt(buf2, 0)

		if err != nil {
			t.Errorf("test %d: error %v", i, err)
			continue
		}

		if n != filesize {
			t.Errorf("test %d: invalid number of bytes returned, want %d, got %d", i, filesize, n)
			continue
		}

		if !bytes.Equal(buf, buf2) {
			t.Errorf("test %d: wrong bytes returned", i)
			continue
		}

		if filesize == 0 {
			n, err = f.ReadAt(buf2, 23)
			if n != 0 {
				t.Errorf("test %d: to read 0 bytes from empty file, got %d", i, n)
			}

			if err == nil {
				t.Errorf("test %d: expected error for empty file at offset 23 not found", i)
			}

			continue
		}

		for j := 0; j < 10; j++ {
			o := rnd.Intn(filesize)
			l := rnd.Intn(filesize - o)

			readatBuf := make([]byte, l)
			n, err = f.ReadAt(readatBuf, int64(o))
			if err != nil {
				t.Errorf("test %d/%d: reading len %v bytes at offset %v failed: %v", i, j, l, o, err)
				continue
			}

			if n != l {
				t.Errorf("test %d/%d: want %d bytes, got %d", i, j, l, n)
				continue
			}

			if !bytes.Equal(readatBuf, buf[o:o+l]) {
				t.Errorf("test %d/%d: wrong bytes returned at offset %v, len %v", i, j, o, l)
			}
		}
	}
}

type Diff struct {
	offset, length int
}

func TestSingleFile(t *testing.T) {
	rnd := rand.New(rand.NewSource(23))

	filesize := 1048576
	f := NewFile(23, filesize, 0)
	buf, err := f.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if len(buf) != filesize {
		t.Fatalf("invalid number of bytes returned, want %d, got %d", filesize, len(buf))
	}

	buf2 := make([]byte, filesize)
	n, err := f.ReadAt(buf2, 0)

	if err != nil {
		t.Fatalf("error %v", err)
	}

	if n != filesize {
		t.Errorf("invalid number of bytes returned, want %d, got %d", filesize, n)
	}

	if !bytes.Equal(buf, buf2) {
		t.Fatalf("wrong bytes returned")
	}

	for j := 0; j < 10; j++ {
		o := rnd.Intn(filesize)
		l := rnd.Intn(filesize - o)

		readatBuf := make([]byte, l)
		n, err = f.ReadAt(readatBuf, int64(o))
		if err != nil {
			t.Errorf("reading len %v bytes at offset %v failed: %v", l, o, err)
			continue
		}

		if n != l {
			t.Errorf("want %d bytes, got %d", l, n)
			continue
		}

		if !bytes.Equal(readatBuf, buf[o:o+l]) {
			t.Errorf("wrong bytes returned at offset %v, len %v", o, l)
		}
	}
}

func TestRandReader(t *testing.T) {
	rd := newRandReader(rand.New(rand.NewSource(23)))

	buf1 := make([]byte, 200)
	n, err := rd.Read(buf1)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(buf1) {
		t.Fatalf("not enough bytes returned, want %d, got %d", len(buf1), n)
	}

	for _, l := range []int{1, 2, 3, 4, 5, 6, 7, 20, 22} {
		buf2 := make([]byte, l)
		rd := newRandReader(rand.New(rand.NewSource(23)))

		for i := 0; i < 2; i++ {
			n, err := rd.Read(buf2)
			if err != nil {
				t.Error(err)
				continue
			}

			if n != len(buf2) {
				t.Errorf("not enough bytes returned, want %d, got %d", len(buf1), n)
				continue
			}

			if !bytes.Equal(buf1[i*l:(i+1)*l], buf2) {
				t.Errorf("bytes not equal: want %02x, got %02x", buf1[i*l:(i+1)*l], buf2)
			}
		}
	}
}

func TestContinuousFileReaderOffsets(t *testing.T) {
	rnd := rand.New(rand.NewSource(23))

	for i, filesize := range testFileSizes {
		f := NewFile(42, filesize, 0)

		buf, err := f.ReadAll()
		if err != nil {
			t.Error(err)
			continue
		}

		if filesize == 0 {
			continue
		}

		for j := 0; j < 10; j++ {
			o := rnd.Intn(filesize)
			l := rnd.Intn(filesize - o)

			buf2 := make([]byte, l)
			crd := ContinuousFileReader(f, int64(o))
			n, err := io.ReadFull(crd, buf2)

			if err != nil {
				t.Error(err)
				continue
			}

			if n != l {
				t.Errorf("test %d/%d: invalid number of bytes: want %d, got %d", i, j, l, n)
				continue
			}

			if !bytes.Equal(buf2, buf[o:o+l]) {
				t.Errorf("test %d/%d: wrong bytes returned at offset %v, len %v", i, j, o, l)
			}
		}
	}
}

func TestContinuousFileReaderReadFull(t *testing.T) {
	for _, filesize := range testFileSizes {
		f := NewFile(42, filesize, 0)

		content, err := f.ReadAll()
		if err != nil {
			t.Error(err)
			continue
		}

		if filesize == 0 {
			continue
		}

		crd := ContinuousFileReader(f, 0)
		buf := make([]byte, filesize)
		n, err := io.ReadFull(crd, buf)

		if err != nil {
			t.Fatal(err)
		}

		if n != filesize {
			t.Fatalf("wrong number of bytes returned, want %d, got %d", filesize, n)
		}

		if !bytes.Equal(buf, content) {
			t.Fatal("wrong bytes returned")
		}
	}
}

func TestContinuousFileReaderReadOffsets(t *testing.T) {
	for _, filesize := range testFileSizes {
		f := NewFile(42, filesize, 0)

		content, err := f.ReadAll()
		if err != nil {
			t.Error(err)
			continue
		}

		if filesize == 0 {
			continue
		}

		buf := make([]byte, 128*1024)
		crd := ContinuousFileReader(f, 0)
		pos := 0
		for pos < filesize {
			n, err := io.ReadFull(crd, buf)

			// allow one io.ErrUnexpectedEOF at the end of the file
			if err == io.ErrUnexpectedEOF && pos+n == filesize {
				err = nil
			}

			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(buf[:n], content[pos:pos+n]) {
				t.Fatalf("filesize %d: wrong bytes returned", filesize)
			}

			pos += n
		}
	}
}

func BenchmarkFileReadContinousThroughput(t *testing.B) {
	filesize := 1<<28 + 1233

	buf := make([]byte, 128*1024)
	f := NewFile(42, filesize, 0)

	t.SetBytes(int64(filesize))
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		rd := ContinuousFileReader(f, 0)

		l := 0
		pos := 0
		for pos < filesize {
			n, err := io.ReadFull(rd, buf)

			// allow one io.ErrUnexpectedEOF at the end of the file
			if err == io.ErrUnexpectedEOF && pos+n == filesize {
				err = nil
			}

			if err != nil {
				t.Fatal(err)
			}

			pos += n
		}

		if pos != filesize {
			t.Fatalf("pos at end is wrong: want %d, got %d", filesize, l)
		}
	}
}

func BenchmarkFileReadContinousReadLatency(t *testing.B) {
	filesize := 1<<28 + 1233

	buf := make([]byte, 128*1024)
	f := NewFile(42, filesize, 0)

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		rd := ContinuousFileReader(f, int64(filesize/2))
		n, err := io.ReadFull(rd, buf)

		if err != nil {
			t.Fatal(err)
		}

		if n != len(buf) {
			t.Fatalf("wrong number of bytes returned, want %d, got %d", len(buf), n)
		}
	}
}

func BenchmarkFileReadAtAll(t *testing.B) {
	filesize := 1 << 28

	buf := make([]byte, 128*1024)
	f := NewFile(42, filesize, 0)

	t.SetBytes(int64(filesize))
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		var pos int64
		for {
			n, err := f.ReadAt(buf, pos)
			if err != nil {
				t.Fatal(err)
			}

			pos += int64(n)
			if n < len(buf) {
				break
			}
		}

		if pos != int64(filesize) {
			t.Fatalf("pos at end is wrong: want %d, got %d", filesize, pos)
		}
	}
}

func BenchmarkFileReadAtOnce(t *testing.B) {
	filesize := 1 << 28

	buf := make([]byte, 128*1024)
	f := NewFile(42, filesize, 0)
	pos := int64(filesize / 2)

	t.SetBytes(int64(filesize))
	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		n, err := f.ReadAt(buf, pos)

		if err != nil {
			t.Fatal(err)
		}

		if n != len(buf) {
			t.Fatalf("invalid number of bytes returned, want %d, got %d", len(buf), n)
		}
	}
}
