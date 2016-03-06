[![Build Status](https://travis-ci.org/restic/fakedatafs.svg?branch=master)](https://travis-ci.org/restic/fakedatafs)
[![Report Card](http://goreportcard.com/badge/restic/fakedatafs)](http://goreportcard.com/report/restic/fakedatafs)

fakedatafs is a file system that generates fake data on demand in a
deterministic way. It is implemented as a FUSE module and can be used to test
backup software.

Build fakedatafs
================

Install Go/Golang (at least version 1.3), then run `go run build.go`,
afterwards you'll find the binary in the current directory:

    $ go run build.go

    $ ./fakedatafs /mnt/dir
    filesystem mounted at /mnt/dir

    $ ls -al /mnt/dir
    total 5078
    -rw-r--r-- 1 root root  15327 Aug 30  1754 file-121872730593067849
    -rw-r--r-- 1 root root  89978 Aug 30  1754 file-1269644873002022781
    -rw-r--r-- 1 root root    879 Aug 30  1754 file-1403895313298597120
    [...]

At the moment, the only tested compiler for restic is the official Go compiler.
Building restic with gccgo may work, but is not supported.
