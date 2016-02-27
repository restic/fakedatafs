package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"github.com/jessevdk/go-flags"
)

var (
	version    = "compiled manually"
	compiledAt = "unknown"
)

// Options are global settings.
type Options struct {
	Version bool `long:"version" short:"V"     description:"print version number"`
	Verbose bool `long:"verbose" short:"v"     description:"be verbose"`

	Seed     int64 `long:"seed"                    default:"23" description:"initial random seed"`
	NumFiles int   `long:"files-per-dir" short:"n" default:"100" description:"number of files per directory"`
	MaxSize  int   `long:"maxsize"       short:"m" default:"100" description:"max individual file size, in KiB"`

	mountpoint string
}

var opts = Options{}
var parser = flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)

var exitRequested = make(chan struct{})

func init() {
	parser.Usage = "mountpoint"
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT)

	go cleanupHandler(c)
}

func cleanupHandler(c <-chan os.Signal) {
	once := &sync.Once{}

	for range c {
		once.Do(func() {
			fmt.Println("Interrupt received, cleaning up")
			close(exitRequested)
		})
	}
}

// V prints debug messages if verbose mode is requested.
func V(format string, data ...interface{}) {
	if opts.Verbose {
		fmt.Printf(format, data...)
	}
}

// M prints a message to stdout.
func M(format string, data ...interface{}) {
	fmt.Printf(format, data...)
}

func mount(opts Options) error {
	fakefs := FakeDataFS{
		Seed:        opts.Seed,
		FilesPerDir: opts.NumFiles,
		MaxSize:     opts.MaxSize * 1024,
	}

	conn, err := fuse.Mount(
		opts.mountpoint,
		fuse.ReadOnly(),
		fuse.FSName("fakedatafs"),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	M("filesystem mounted at %v\n", opts.mountpoint)

	serveErrCh := make(chan error, 2)
	go func() {
		V("serving\n")
		err := fs.Serve(conn, fakefs)
		if err != nil {
			serveErrCh <- err
		}
		<-conn.Ready
		serveErrCh <- conn.MountError
	}()

	for {
		select {
		case err := <-serveErrCh:
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return err
		case <-exitRequested:
			fmt.Printf("umounting...\n")
			return fuse.Unmount(opts.mountpoint)
		}
	}
}

func main() {
	args, err := parser.Parse()
	if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
		parser.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	if err != nil {
		os.Exit(1)
	}

	if opts.Version {
		fmt.Printf("version %v, compiled at %v using %v\n", version, compiledAt, runtime.Version())
		return
	}

	if len(args) == 0 {
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	opts.mountpoint = args[0]
	err = mount(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
}
