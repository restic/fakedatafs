package main

import "bazil.org/fuse/fs"

// FakeDataFS is a filesystem filled with fake data.
type FakeDataFS struct {
	Seed        int64
	MaxSize     int
	FilesPerDir int
}

// Root returns the root node of the filesystem.
func (f FakeDataFS) Root() (fs.Node, error) {
	return NewDir(f.Seed, "/", f.FilesPerDir, f.MaxSize), nil
}
