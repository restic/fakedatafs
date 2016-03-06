package main

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/jacobsa/fuse/fuseops"
)

type cacheKey struct {
	Inode  fuseops.InodeID
	Offset int64
}

type cacheEntry struct {
	file io.Reader
	t    time.Time
}

// Cache holds a list of recently read files.
type Cache struct {
	entries map[cacheKey]cacheEntry
	m       sync.Mutex
}

func newCache() *Cache {
	return &Cache{
		entries: make(map[cacheKey]cacheEntry),
	}
}

// Get retrieves and removes an entry from the cache.
func (c *Cache) Get(inode fuseops.InodeID, off int64) (io.Reader, error) {
	c.m.Lock()
	defer c.m.Unlock()

	key := cacheKey{Inode: inode, Offset: off}
	entry, ok := c.entries[key]
	if !ok {
		return nil, errors.New("not found")
	}

	delete(c.entries, key)

	return entry.file, nil
}

// Put stores an entry in the cache.
func (c *Cache) Put(inode fuseops.InodeID, off int64, rd io.Reader) {
	c.m.Lock()
	defer c.m.Unlock()

	key := cacheKey{Inode: inode, Offset: off}
	_, ok := c.entries[key]
	if ok {
		return
	}

	entry := cacheEntry{
		file: rd,
		t:    time.Now(),
	}

	c.entries[key] = entry
}

const cacheTimeout = 20 * time.Second

// Cleanup removes old entries from the cache.
func (c *Cache) Cleanup() {
	c.m.Lock()
	defer c.m.Unlock()

	for key, entry := range c.entries {
		if time.Since(entry.t) > cacheTimeout {
			delete(c.entries, key)
		}
	}
}
