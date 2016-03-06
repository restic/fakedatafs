package main

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
	"path"

	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

// seedForPath returns a new seed for an item of typ at path.
func seedForPath(parentSeed int64, tpe string, path string) (seed int64) {
	s := fmt.Sprintf("seed-%16x/%s/%s", seed, tpe, path)
	hash := sha1.Sum([]byte(s))
	for i := 0; i < 8; i++ {
		shift := uint(i) * 8
		seed |= int64(hash[i]) << shift
	}

	return seed
}

// Dir is a directory containing fake data.
type Dir struct {
	seed    int64
	path    string
	maxSize int

	entries []fuseutil.Dirent
	// inodes  map[fuseops.InodeID]string
}

// NewDir initializes a directory.
func NewDir(fs *FakeDataFS, seed int64, dir string, numEntries, maxSize int) *Dir {
	d := Dir{
		seed:    seed,
		path:    dir,
		maxSize: maxSize,
		entries: make([]fuseutil.Dirent, numEntries),
		// inodes:  make(map[fuseops.InodeID]string),
	}

	V("generate dir %v with %d entries\n", d, numEntries)
	rnd := rand.New(rand.NewSource(d.seed))
	for i := range d.entries {
		name := fmt.Sprintf("file-%d", rnd.Int())
		inode := inodePath(path.Join(d.path, name))

		d.entries[i] = fuseutil.Dirent{
			Offset: fuseops.DirOffset(i+1),
			Name:   name,
			Type:   fuseutil.DT_File,
			Inode:  inode,
		}

		p := path.Join(d.path, name)
		fileSeed := seedForPath(d.seed, "file", p)
		size := rnd.Intn(d.maxSize)
		f := NewFile(fileSeed, size, inodePath(p))

		fs.entries[inode] = Entry{
			File: f,
			Attr: fuseops.InodeAttributes{
				Nlink: 1,
				Mode: 0644,
				Size: uint64(size),
			},
		}
	}

	return &d
}

func (d Dir) String() string {
	return fmt.Sprintf("<Dir %v [seed %v]>", d.path, d.seed)
}

// inode returns the inode for a given file name.
func (d Dir) inode(name string) fuseops.InodeID {
	return inodePath(path.Join(d.path, name))
}

// ReadDir returns the entries of this directory.
func (d Dir) ReadDir(dst []byte, offset int) (n int) {
	for _, entry := range d.entries[offset:] {
		written := fuseutil.WriteDirent(dst[n:], entry)
		if written == 0 {
			break
		}

		n += written
	}

	return n
}
