package main

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
	"os"
	"path"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
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

// Entries contains a list of files for a directory.
type Entries map[string]*File

// Dir is a directory containing fake data.
type Dir struct {
	seed       int64
	path       string
	maxSize    int
	numEntries int
	entries    *Entries
}

// NewDir initializes a directory.
func NewDir(seed int64, path string, numEntries, maxSize int) Dir {
	// source := rand.NewSource(seed)
	d := Dir{
		seed:       seed,
		path:       path,
		maxSize:    maxSize,
		numEntries: numEntries,
	}

	entries := d.generateEntries()
	d.entries = &entries

	V("generated dir %v\n", d)

	return d
}

func (d Dir) String() string {
	return fmt.Sprintf("<Dir %v [seed %v]>", d.path, d.seed)
}

// file returns the file for the given name.
func (d Dir) file(name string) *File {
	p := path.Join(d.path, name)
	fileSeed := seedForPath(d.seed, "file", p)

	size := rand.Intn(d.maxSize)

	return NewFile(fileSeed, size, inodePath(p))
}

// generateEntries returns a slice with count file entries for a directory.
func (d Dir) generateEntries() (entries Entries) {
	rand := rand.New(rand.NewSource(d.seed))
	entries = make(map[string]*File, d.numEntries)

	for i := 0; i < d.numEntries; i++ {
		name := fmt.Sprintf("file-%d", rand.Int())
		entries[name] = d.file(name)
	}

	return entries
}

// Attr returns the attributes for this directory.
func (d Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	V("Attr(%v)\n", d)

	a.Inode = inodePath(d.path)
	a.Mode = os.ModeDir | 0555
	return nil
}

// ReadDirAll returns the entries of this directory.
func (d Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	V("ReadDirAll(%v)\n", d)

	ret := make([]fuse.Dirent, 0, d.numEntries)
	for name, file := range *d.entries {
		ret = append(ret, fuse.Dirent{
			Inode: file.Inode,
			Type:  fuse.DT_File,
			Name:  name,
		})
	}

	return ret, nil
}

// Lookup returns the file for the given name in d.
func (d Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	V("Lookup(%v, %v)\n", d, name)

	if node, ok := (*d.entries)[name]; ok {
		return node, nil
	}

	return nil, fuse.ENOENT
}
