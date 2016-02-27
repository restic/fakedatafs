package main

import (
	"io"
	"io/ioutil"
	"testing"
)

var testFileSizes = []int{0, 100, 200, 500, 1024, 7666, 1 << 20}

func TestFile(t *testing.T) {
	for i, filesize := range testFileSizes {
		f := NewFile(23, filesize, 23)

		n, err := io.Copy(ioutil.Discard, f)
		if err != nil {
			t.Errorf("test %d: error %v", i, err)
			continue
		}

		if n != int64(filesize) {
			t.Errorf("test %d: invalid number of bytes returned, want %d, got %d", i, filesize, n)
			continue
		}
	}
}
