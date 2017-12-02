package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"golang.org/x/net/context"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseutil"
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
	Debug   bool `long:"debug"                 description:"output debug messages"`

	Seed     int64 `long:"seed"                    default:"23" description:"initial random seed"`
	NumFiles int   `long:"files-per-dir" short:"n" default:"100" description:"number of files per directory"`
	MaxSize  int   `long:"maxsize"       short:"m" default:"100" description:"max individual file size, in KiB"`

	mountpoint string
}

var opts = Options{}
var parser = flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)

var ctx context.Context

func init() {
	parser.Usage = "mountpoint"

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT)

	go func() {
		once := &sync.Once{}

		for range c {
			once.Do(func() {
				fmt.Println("Interrupt received, cleaning up")
				cancel()
			})
		}
	}()
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

func mount(opts Options) (*fuse.MountedFileSystem, error) {
	fakefs, err := NewFakeDataFS(ctx, opts.Seed, opts.MaxSize*1024, opts.NumFiles)
	if err != nil {
		return nil, err
	}

	cfg := &fuse.MountConfig{
		FSName:      "fakedatafs",
		ReadOnly:    true,
		ErrorLogger: log.New(os.Stderr, "ERROR: ", log.LstdFlags),
	}

	if opts.Debug {
		cfg.DebugLogger = log.New(os.Stderr, "DEBUG: ", log.LstdFlags)
	}

	fs, err := fuse.Mount(
		opts.mountpoint,
		fuseutil.NewFileSystemServer(fakefs),
		cfg,
	)
	if err != nil {
		return nil, err
	}

	M("filesystem mounted at %v\n", opts.mountpoint)
	return fs, nil
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
	fs, err := mount(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	fs.Join(ctx)

	err = fuse.Unmount(fs.Dir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(3)
	}
}
