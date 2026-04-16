// Command goavif-decode reads an AVIF file and writes a PNG to stdout.
//
// Usage: goavif-decode input.avif > output.png
//
// Supports single stills (common path) plus AVIS sequences (where the
// first frame is emitted). 8-, 10- and 12-bit bit depths, with or
// without alpha, are accepted.
package main

import (
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"

	"github.com/KarpelesLab/goavif"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: goavif-decode <input.avif>")
		os.Exit(2)
	}
	in, err := os.Open(os.Args[1])
	must(err)
	defer in.Close()
	img, err := goavif.Decode(in)
	if errors.Is(err, goavif.ErrUnsupported) {
		fmt.Fprintln(os.Stderr, "goavif-decode:", err)
		os.Exit(1)
	}
	must(err)
	must(png.Encode(os.Stdout, img))
}

func must(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "goavif-decode:", err)
	if _, ok := err.(*os.PathError); ok {
		os.Exit(2)
	}
	os.Exit(1)
}

var _ = io.Discard // silence import if not used elsewhere
