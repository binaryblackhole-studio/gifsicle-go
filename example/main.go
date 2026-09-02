// Command example optimizes a GIF with gifsicle-go and writes the result
// to stdout.
//
// Usage:
//
//	example [flags] [input.gif]    reads stdin when no file is given
//
// It is also the payload of the docker image built by
// Dockerfile.example:
//
//	docker run gifsicle-go-example input.gif > output.gif
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	gifsicle "github.com/binaryblackhole-studio/gifsicle-go"
)

func main() {
	log.SetFlags(0)
	// accept gifsicle-style glued flags: -O2 means -O 2
	args := make([]string, 0, len(os.Args))
	for _, a := range os.Args[1:] {
		if len(a) == 3 && a[:2] == "-O" && a[2] >= '0' && a[2] <= '9' {
			args = append(args, "-O", a[2:])
		} else {
			args = append(args, a)
		}
	}
	fs := flag.NewFlagSet("example", flag.ExitOnError)
	level := fs.Int("O", 3, "optimization level: 0 (none), 1, 2 or 3")
	lossy := fs.Bool("lossy", false, "also apply lossy compression (gifsicle --lossy)")
	loop := fs.Bool("loop", false, "force the output to loop forever")
	fs.Parse(args)

	in := io.Reader(os.Stdin)
	if fs.NArg() > 1 {
		log.Fatalf("usage: %s [-O level] [-lossy] [-loop] [input.gif]", os.Args[0])
	}
	if name := fs.Arg(0); name != "" && name != "-" {
		f, err := os.Open(name)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		in = f
	}

	data, err := io.ReadAll(in)
	if err != nil {
		log.Fatal(err)
	}
	g, err := gifsicle.Open(data)
	if err != nil {
		log.Fatal(err)
	}

	var opts []gifsicle.ExportOption
	if *level > 0 {
		if *level > 3 {
			log.Fatalf("invalid optimization level -O%d (0..3)", *level)
		}
		opts = append(opts, gifsicle.Optimize(gifsicle.OptimizationLevel(*level)))
	}
	if *lossy {
		opts = append(opts, gifsicle.Lossy())
	}
	if *loop {
		opts = append(opts, gifsicle.Loop())
	}

	out, err := g.Export(opts...)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		log.Fatal(err)
	}
	if n := g.Frames(); n > 0 {
		if _, err := fmt.Fprintf(os.Stderr, "example: %d frame(s), %d -> %d bytes\n",
			n, len(data), len(out)); err != nil {
			log.Fatal(err)
		}
	}
}