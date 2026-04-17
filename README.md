# goavif

Pure-Go AVIF image codec — container and AV1 bitstream. No cgo, no third-party
runtime dependencies in the core codec path.

> **Status: feature-complete for the common still-image + image-sequence
> decode path. The encoder writes AVIF stills and intra-only AVIS
> sequences end-to-end.**
>
> Decode covers 8/10/12-bit, alpha, 4:2:0 / 4:2:2 / 4:4:4 chroma, AVIS
> sequences, grid (tiled) primary items, irot/imir/clap transform
> properties, and the full intra-only AV1 feature set (13 intra modes
> with extended neighbors, CDEF, deblocking, loop restoration, film
> grain).
>
> Encode supports 8/10/12-bit color + optional alpha, all three chroma
> subsampling modes, grayscale inputs, auto-padding for non-64-aligned
> dimensions, and AVIS sequences via EncodeAll. Intra mode search tries
> 13 modes (DC/V/H/Paeth/Smooth*/all six directional) and picks lowest
> SAD; adaptive 32→16 partition split for high-detail content; full 2D
> forward transforms with Golomb-tail coefficient coding.
>
> Still TODO: inter prediction for AVIS (decoder returns degraded output
> with ErrInterPredictionNotImplemented today); bit-exact interop with
> dav1d/libavif; clap (clean-aperture) crop transforms. See
> [ROADMAP.md](ROADMAP.md).

## Install

```
go get github.com/KarpelesLab/goavif
```

## Usage

### Decode a single AVIF still

```go
import (
    "image"
    _ "github.com/KarpelesLab/goavif" // registers "avif" with image.Decode
    "os"
)

f, _ := os.Open("example.avif")
defer f.Close()
img, _, err := image.Decode(f)
// img is image.YCbCr (8-bit) or image.RGBA64 (10/12-bit HDR);
// for files with alpha you get image.NRGBA / NRGBA64.
```

Or direct:

```go
import "github.com/KarpelesLab/goavif"

img, err := goavif.Decode(r)
```

### Decode an AVIS image sequence

```go
frames, delays, err := goavif.DecodeAll(r)
// delays[i] is the presentation duration for frames[i].
// Intra-only AVIS decodes end-to-end; inter-predicted non-sync
// samples return ErrInterPredictionNotImplemented with previous-
// frame fill.
```

### Encode

```go
err := goavif.Encode(w, img, &goavif.Options{Quality: 90})
```

Supported inputs: `image.RGBA`, `image.NRGBA`, `image.YCbCr` (full color);
`image.Gray`, `image.Gray16` (monochrome); and anything else via the
generic `image.Image` `At()` fallback. Non-opaque alpha is detected
automatically and written as a second AV1 item; set `opts.Alpha = true`
to force-emit the alpha even when every pixel is opaque.

The encoder currently produces 8-bit 4:2:0 output (profile 0). 10/12-bit
HBD encoding is on the roadmap; for now HBD inputs are downsampled.

### Encode from the command line

```
go run ./cmd/goavif-encode -q 90 input.png > output.avif
go run ./cmd/goavif-encode -q 90 -alpha input.png > output.avif
```

### Read image metadata

```go
cfg, _, err := image.DecodeConfig(r)
// cfg.Width, cfg.Height populated from ispe; cfg.ColorModel matches the
// bit depth (color.RGBAModel / color.RGBA64Model).
```

### Inspect an AVIF with the CLI

```
go run ./cmd/goavif-info some.avif
```

Prints the ftyp brands, item list, properties (ispe/av1C/pixi/colr/…),
property associations, and the parsed AV1 sequence header.

### Work with the container directly

The `isobmff` subpackage exposes the box tree:

```go
import "github.com/KarpelesLab/goavif/isobmff"

ct, err := isobmff.ParseContainer(data)          // parse
bits, err := ct.ItemData(ct.PrimaryItemID())     // extract AV1 bitstream

// Build a new AVIF container from raw AV1 bytes:
built, _ := isobmff.BuildStillImage(isobmff.StillImage{
    Width: 320, Height: 240, BitDepth: 8,
    ChromaSubsamplingX: 1, ChromaSubsamplingY: 1,
    ConfigOBUs:   seqHeaderOBU,
    AV1Bitstream: encodedFrameOBUs,
})
out, _ := built.Encode()
```

## Architecture

```
goavif/
├── avif.go              # public API: Decode, DecodeConfig, DecodeAll, Encode
├── alpha.go             # auxl iref + auxC URN lookup, NRGBA compositing
├── sequence.go          # AVIS sample-table walker, DecodeAll
├── isobmff/             # ISOBMFF container: boxes, meta, moov, stbl
├── av1/
│   ├── bitio/           # f(n)/su/uvlc/leb128 reader + writer
│   ├── obu/             # sequence + frame header parse + write
│   ├── entropy/         # range-coded CDF decoder + encoder
│   ├── predict/         # intra prediction (uint8 + uint16)
│   ├── transform/       # inverse + forward DCT/ADST/IDTX, all sizes
│   ├── quant/           # de/quant tables + signed rounding
│   ├── loopfilter/      # deblocking (uint8 + uint16)
│   ├── cdef/            # constrained directional enhancement filter
│   ├── lr/              # loop restoration (Wiener + SGR)
│   ├── filmgrain/       # LFSR + scaling LUT + AR shaping + patch tiling
│   ├── decoder/         # tile decoder, partition walker, FrameState
│   └── encoder/         # tile writer (baseline)
├── colorspace/          # YUV↔RGB, CICP matrices, Studio/Full range
└── cmd/goavif-info/     # CLI inspector
```

## Goals

- Full decode and encode of AVIF (still + image sequences).
- 8-bit 4:2:0, 10/12-bit HDR, alpha channel.
- Pure Go — no cgo, no external binaries.
- Spec-faithful implementation: variable names and structure track the AV1
  bitstream specification so the code stays auditable.
- Standard-library only in the core codec.

## Non-goals (for now)

- GPU / hardware acceleration.
- Streaming / progressive decode.
- AVIF grid / derived images (deferred until after base decode is solid).
- DRM.

## Development

```
go test ./...
go vet ./...
```

## License

See [LICENSE](LICENSE).
