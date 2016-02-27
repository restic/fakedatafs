fakedatafs is a file system that generates fake data on demand in a
deterministic way. It is implemented as a FUSE module and can be used to test
backup software.

Build restic
============

Install Go/Golang (at least version 1.3), then run `go run build.go`,
afterwards you'll find the binary in the current directory:

    $ go run build.go

    $ ./fakedatafs /mnt/dir

At the moment, the only tested compiler for restic is the official Go compiler.
Building restic with gccgo may work, but is not supported.
