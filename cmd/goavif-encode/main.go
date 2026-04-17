// Command goavif-encode reads a PNG/JPEG from stdin (or a file) and
// writes an AVIF to stdout (or a file).
//
// Usage:
//   goavif-encode input.png > output.avif
//   goavif-encode -q 90 input.jpg > output.avif
//   goavif-encode -q 90 -alpha input.png > output.avif
//   goavif-encode -q 95 -bit-depth 10 -subsampling 444 input.png > output.avif
//
// The encoder supports 8/10/12-bit color + alpha via separate AV1
// items, picks intra modes by SAD search (DC / V / H / Paeth /
// Smooth*), adaptively splits 32×32 blocks into 16×16 for high-detail
// content, emits full 2D forward transforms with Golomb-tail
// coefficient coding, and handles 4:2:0 / 4:2:2 / 4:4:4 chroma.
// Decodes round-trip through goavif.Decode.
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
	quality := flag.Int("q", 50, "encode quality (1..100); ignored when -target-bytes is set")
	alpha := flag.Bool("alpha", false, "force emission of an alpha auxiliary item even when all pixels are opaque")
	bitDepth := flag.Int("bit-depth", 0, "bit depth: 8, 10, or 12 (0 = auto from input)")
	subsampling := flag.String("subsampling", "", "chroma subsampling: 420, 422, 444 (default: from input YCbCr, else 4:2:0)")
	targetBytes := flag.Int("target-bytes", 0, "target file size in bytes; enables rate-control Q-bisection loop")
	speed := flag.Int("speed", 0, "encode speed 0..10 (0=slowest/best, 10=fastest/worst); affects ME search range")
	filmGrain := flag.Int("film-grain", 0, "film grain luma strength 0..255 (0=off, 8..32=subtle, 48..64=heavy)")
	outPath := flag.String("o", "", "output file path (default stdout)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: goavif-encode [-q QUALITY | -target-bytes N] [-alpha] [-bit-depth N] [-subsampling S] [-speed N] [-film-grain N] [-o OUT] input.{png,jpg}")
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

	if *filmGrain < 0 || *filmGrain > 255 {
		fmt.Fprintf(os.Stderr, "goavif-encode: invalid -film-grain %d (want 0..255)\n", *filmGrain)
		os.Exit(2)
	}
	opts := &goavif.Options{
		Quality:           *quality,
		Alpha:             *alpha,
		TargetBytes:       *targetBytes,
		Speed:             *speed,
		FilmGrainStrength: uint8(*filmGrain),
	}
	switch *bitDepth {
	case 0:
		// Auto.
	case 8, 10, 12:
		opts.BitDepth = *bitDepth
	default:
		fmt.Fprintf(os.Stderr, "goavif-encode: invalid -bit-depth %d (want 8, 10, or 12)\n", *bitDepth)
		os.Exit(2)
	}
	switch *subsampling {
	case "":
		// Auto.
	case "420":
		opts.ChromaSubsampling = goavif.Chroma420
	case "422":
		opts.ChromaSubsampling = goavif.Chroma422
	case "444":
		opts.ChromaSubsampling = goavif.Chroma444
	default:
		fmt.Fprintf(os.Stderr, "goavif-encode: invalid -subsampling %q (want 420, 422, or 444)\n", *subsampling)
		os.Exit(2)
	}
	err = goavif.Encode(out, img, opts)
	must(err)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "goavif-encode:", err)
		os.Exit(1)
	}
}
