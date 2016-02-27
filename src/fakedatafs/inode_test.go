package main

import "testing"

var inodePathTests = []struct {
	path  string
	inode uint64
}{
	{"/", 1251674434},
	{"/foo/bar", 902704296},
}

func TestInodePath(t *testing.T) {
	for i, test := range inodePathTests {
		inode := inodePath(test.path)

		if inode != test.inode {
			t.Errorf("test %d: wrong inode returned, want %d, got %d", i, test.inode, inode)
		}
	}
}
