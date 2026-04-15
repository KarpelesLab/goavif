# goavif

Pure-Go AVIF image codec — container and AV1 bitstream. No cgo, no third-party
runtime dependencies in the core codec path.

> **Status: early development.** The ISOBMFF container is implemented;
> `DecodeConfig` returns valid dimensions. `Decode` and `Encode` are stubs.
> See [ROADMAP.md](ROADMAP.md) for the phase plan and progress.

## Install

```
go get github.com/KarpelesLab/goavif
```

## Usage

### Read image metadata (works today)

```go
import (
    "image"
    _ "github.com/KarpelesLab/goavif" // registers "avif" with the image pkg
)

cfg, format, err := image.DecodeConfig(r)
// cfg.Width, cfg.Height populated from the AVIF container
```

### Decode / encode (coming)

Both `goavif.Decode(r)` and `goavif.Encode(w, img, opts)` are currently stubs
that return `goavif.ErrUnsupported`. Track progress in the roadmap.

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
