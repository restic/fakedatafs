package main

import "crypto/sha1"

// inodePath returns a deterministic 32bit inode for the path p.
func inodePath(p string) (inode uint64) {
	hash := sha1.Sum([]byte(p))
	for i := 0; i < 4; i++ {
		shift := uint(i) * 8
		inode |= uint64(hash[i]) << shift
	}

	return inode
}
