package main

import "bazil.org/fuse/fs"

// ensure that FakeDataFS implements fuse/fs.FS
var _ fs.FS = FakeDataFS{}
