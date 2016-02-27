package main

import (
	"io"
	"math/rand"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

// File represents fake data with a specific seed.
type File struct {
	Seed int64
	Size  int
	Inode uint64
}

// NewFile initializes a new file with the given seed.
func NewFile(seed int64, size int, inode uint64) *File {
	return &File{
		Seed:  seed,
		Size:  size,
		Inode: inode,
	}
}

// ReadAll returns the content of the file.
func (f File) ReadAll() ([]byte, error) {
	buf := make([]byte, f.Size)
	src := rand.New(rand.NewSource(f.Seed))
	_, err := io.ReadFull(src, buf)
	return buf, err
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
