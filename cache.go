package main

import (
	"errors"
	"io"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/jacobsa/fuse/fuseops"
)

type cacheKey struct {
	Inode  fuseops.InodeID
	Offset int64
}

type cacheEntry struct {
	rd io.Reader
	t  time.Time
}

// Cache holds a list of recently read files.
type Cache struct {
	entries map[cacheKey]cacheEntry
	m       sync.Mutex
}

func newCache(ctx context.Context) *Cache {
	c := &Cache{
		entries: make(map[cacheKey]cacheEntry),
	}

	go c.cleanup(ctx)

	return c
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

	return entry.rd, nil
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
		rd: rd,
		t:  time.Now(),
	}

	c.entries[key] = entry
}

const (
	cacheTimeout = 20 * time.Second
	cacheTicker  = 5 * time.Second
)

// cleanup removes old entries from the cache.
func (c *Cache) cleanup(ctx context.Context) {
	ticker := time.NewTicker(cacheTicker)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			n := 0
			c.m.Lock()
			for key, entry := range c.entries {
				if time.Since(entry.t) > cacheTimeout {
					delete(c.entries, key)
					n++
				}
			}
			c.m.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
