// Command goavif-encode reads a PNG/JPEG from stdin (or a file) and
// writes an AVIF to stdout (or a file).
//
// Usage:
//   goavif-encode input.png > output.avif
//   goavif-encode -q 70 input.jpg > output.avif
//
// The baseline encoder is "minimum viable": output pixels are a
// constant mid-grey regardless of input. Residual coefficient coding
// (which would preserve input content) is pending Phase 6+. The file
// still round-trips structurally through goavif.Decode and any other
// AV1 reader that understands the produced sequence/frame header
// layout.
package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/KarpelesLab/goavif"
)

func main() {
	quality := flag.Int("q", 50, "encode quality (1..100)")
	outPath := flag.String("o", "", "output file path (default stdout)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: goavif-encode [-q QUALITY] [-o OUT] input.{png,jpg}")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	in, err := os.Open(flag.Arg(0))
	must(err)
	defer in.Close()

	img, _, err := image.Decode(in)
	must(err)

	out := os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		must(err)
		defer f.Close()
		out = f
	}
	err = goavif.Encode(out, img, &goavif.Options{Quality: *quality})
	must(err)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "goavif-encode:", err)
		os.Exit(1)
	}
}
