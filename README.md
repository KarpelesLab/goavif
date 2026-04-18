## goavif

Pure-Go AVIF image codec — container and AV1 bitstream. No cgo, no third-party
runtime dependencies in the core codec path.

> **Status: feature-complete for the AVIF / AVIS surface. Both encode and
> decode handle stills and animated sequences end-to-end.**
>
> **Decode** covers 8/10/12-bit, alpha, monochrome, 4:2:0 / 4:2:2 / 4:4:4
> chroma, AVIS image sequences with single-reference translational inter
> prediction (sub-pel 8-tap filter, edge-clamped MC), grid (tiled) primary
> items, irot / imir / clap transform properties, HDR (CICP matrix + color
> range), CDEF, deblocking, loop restoration primitives, film-grain
> synthesis, and the full intra AV1 mode set (DC / V / H / Paeth /
> Smooth* / all six directional predictors with extended neighbors).
>
> **Encode** supports 8/10/12-bit color + optional alpha at 4:2:0 / 4:2:2 /
> 4:4:4, grayscale (monochrome), intra mode search over 13 modes, adaptive
> 32→16 partition split, Golomb-tail coefficient coding, grid images,
> target-size rate control, film-grain emission, and AVIS sequences with
> real inter-frame compression: per-block motion estimation with sub-pel
> refinement, adaptive 32→16 partition on high-residual blocks, 8/10/12-bit
> and monochrome inter paths.
>
> Known follow-ups (tracked in [ROADMAP.md](ROADMAP.md)): compound /
> warped / non-identity-global motion; per-unit loop-restoration signalling;
> full rate-distortion mode decision with bit-cost modeling; film-grain
> *estimation* from the input; SIMD asm; parallel tile decode/encode.

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
// Sync and non-sync samples both decode through the inter path;
// unsupported inter modes (compound, warp) fall back to repeating
// the previous frame and signal ErrInterPredictionNotImplemented.
```

### Encode a still

```go
err := goavif.Encode(w, img, &goavif.Options{Quality: 90})
```

Supported inputs: `image.RGBA`, `image.NRGBA`, `image.YCbCr`, `image.RGBA64`,
`image.NRGBA64` (full color); `image.Gray`, `image.Gray16` (monochrome); and
anything else via the generic `image.Image` `At()` fallback. Non-opaque alpha
is detected automatically and written as a second AV1 item; set
`opts.Alpha = true` to force-emit alpha even when every pixel is opaque.

Key `Options` fields:

| Field | Meaning |
|-------|---------|
| `Quality` | 0..100, higher = better (default ≈ 50). |
| `Lossless` | Forces lossless. Overrides `Quality`. |
| `BitDepth` | 8, 10, or 12 (0 = auto from input type). |
| `ChromaSubsampling` | `Chroma420` / `Chroma422` / `Chroma444` / `Chroma400`. |
| `Alpha` | Force-emit alpha aux item. |
| `TargetBytes` | If non-zero, Encode runs a Q-bisection loop to land within ±10 % of the target size. |
| `Speed` | 0..10. Higher = faster encode, narrower inter-frame ME search. |
| `FilmGrainStrength` | 0..255. Non-zero emits `film_grain_params` so the decoder overlays synthetic grain. |
| `InterEnabled`, `KeyFrameInterval` | Enable inter-frame prediction for AVIS sequences (see below). |

### Encode an AVIS sequence

```go
err := goavif.EncodeAll(w, frames, delays, &goavif.Options{
    Quality:          85,
    InterEnabled:     true,
    KeyFrameInterval: 30, // 1 keyframe every 30 frames
})
```

Frame 0 is always a sync keyframe. With `InterEnabled` + `KeyFrameInterval > 1`
the encoder runs motion estimation per 32×32 block (adaptive split to 16×16 on
high-residual regions), refines to quarter-pel, and writes a real inter-frame
bitstream against the previously decoded frame. Works at 8/10/12-bit and all
four chroma modes (incl. monochrome). Arbitrary-dim input is auto-padded to a
64-multiple; the container's tkhd records the original dims and `DecodeAll`
crops back on read.

### Encode from the command line

```
go run ./cmd/goavif-encode -q 90 input.png > output.avif
go run ./cmd/goavif-encode -q 90 -alpha input.png > output.avif
go run ./cmd/goavif-encode -q 95 -bit-depth 10 -subsampling 444 input.png > out.avif
go run ./cmd/goavif-encode -target-bytes 20000 input.png > sized.avif
go run ./cmd/goavif-encode -film-grain 32 input.png > grainy.avif
```

Flags: `-q`, `-target-bytes`, `-speed`, `-bit-depth`, `-subsampling`,
`-alpha`, `-film-grain`, `-o`.

### Decode from the command line

```
go run ./cmd/goavif-decode input.avif > output.png
```

### Read image metadata

```go
cfg, _, err := image.DecodeConfig(r)
// cfg.Width, cfg.Height from ispe; cfg.ColorModel reflects the bit depth
// (YCbCrModel for 8-bit color, RGBA64Model for HDR, Gray for monochrome,
// NRGBAModel when alpha is present).
```

### Inspect an AVIF with the CLI

```
go run ./cmd/goavif-info some.avif
```

Prints the ftyp brands, item list, properties (ispe / av1C / pixi / colr /
pasp / irot / imir / clap / auxC / …), property associations, and the parsed
AV1 sequence header.

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

Also available: `BuildGrid` (tiled primary item + dimg iref) and
`BuildSequence` (AVIS image track with stts / stsc / stsz / stco / stss).

## Architecture

```
goavif/
├── avif.go              # public API: Decode, DecodeConfig, DecodeAll, Encode, EncodeGrid
├── alpha.go             # auxl iref + auxC URN lookup, NRGBA compositing
├── sequence.go          # AVIS sample-table walker, DecodeAll, EncodeAll
├── grid.go              # grid image encode / decode helpers
├── isobmff/             # ISOBMFF container: boxes, meta, moov, stbl, tkhd
├── av1/
│   ├── bitio/           # f(n) / su / uvlc / leb128 reader + writer
│   ├── obu/             # sequence + frame header parse + write (intra + inter + AVIS + mono)
│   ├── entropy/         # range-coded CDF decoder + encoder
│   ├── predict/         # intra prediction + 8-tap inter interpolation (uint8 + uint16)
│   ├── transform/       # inverse + forward DCT / ADST / IDTX, all sizes
│   ├── quant/           # de/quant tables + signed rounding
│   ├── loopfilter/      # deblocking (uint8 + uint16)
│   ├── cdef/            # constrained directional enhancement filter
│   ├── lr/              # loop restoration (Wiener + SGR, uint8 + uint16)
│   ├── filmgrain/       # LFSR + scaling LUT + AR shaping + patch tiling
│   ├── decoder/         # tile decoder, partition walker, FrameState (intra + inter)
│   └── encoder/         # tile writer (intra + inter), motion estimation
├── colorspace/          # YUV↔RGB, CICP matrices, Studio/Full range (8-bit + HBD)
└── cmd/
    ├── goavif-info/     # container / sequence-header inspector
    ├── goavif-decode/   # AVIF → PNG CLI
    └── goavif-encode/   # PNG/JPEG → AVIF CLI
```

## Goals

- Full decode and encode of AVIF (still + image sequences).
- 8/10/12-bit, 4:2:0 / 4:2:2 / 4:4:4, monochrome, alpha, HDR.
- Inter-frame compression for AVIS (single-ref translational, sub-pel).
- Pure Go — no cgo, no external binaries.
- Spec-faithful implementation: variable names and structure track the AV1
  bitstream specification so the code stays auditable.
- Standard-library only in the core codec.

## Non-goals (for now)

- GPU / hardware acceleration.
- Streaming / progressive decode.
- Compound / warped / non-identity-global motion (decoder + encoder).
- DRM.

## Development

```
go test ./...
go vet ./...
go test -bench=. ./...
```

## License

See [LICENSE](LICENSE).
