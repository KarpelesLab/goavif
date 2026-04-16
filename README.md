# goavif

Pure-Go AVIF image codec — container and AV1 bitstream. No cgo, no third-party
runtime dependencies in the core codec path.

> **Status: feature-complete for the common still-image + image-sequence
> decode path; baseline encoder lands an Encode→Decode round trip end-to-end.**
> 8/10/12-bit, alpha, and AVIS sequences all work on the decode side.
> The encoder is "minimum viable" — it emits all-skip DC_PRED so output
> pixels are a constant mid-grey. Residual coefficient coding + real mode
> decision are follow-ups. See [ROADMAP.md](ROADMAP.md) for the phase
> plan and progress.

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

### Encode (baseline)

```go
err := goavif.Encode(w, img, &goavif.Options{Quality: 50})
```

The baseline encoder produces a valid AVIF container that round-trips
through `goavif.Decode`. Output pixels are currently mid-grey
regardless of input — residual coefficient coding is the next piece.

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
