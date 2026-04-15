# goavif roadmap

Phased plan for the pure-Go AVIF codec. Each phase is independently
shippable and testable. Checkboxes track completion within a phase.

## Phase 1 — ISOBMFF container read/write ✅

Parse and serialize the AVIF container for still images.

- [x] Generic box reader/writer with largesize + uuid support
- [x] Core top-level boxes: ftyp, meta, hdlr, pitm, iloc, iinf/infe, iprp
      (ipco + ipma), iref, mdat
- [x] Item property boxes: ispe, pixi, av1C, colr (nclx/rICC/prof), pasp,
      irot, imir, auxC, clap
- [x] High-level `Container` with Parse/Encode and iloc-aware `ItemData`
- [x] `BuildStillImage` helper that patches iloc offsets after layout
- [x] Public API stubs and `image.RegisterFormat` registration
- [x] Roundtrip tests (typed boxes + full container)

## Phase 2 — AV1 decoder: OBU parsing + intra-only still frames

Minimum viable decoder: decode a single keyframe from an AVIF still to
8-bit 4:2:0 YCbCr, including deblocking and CDEF.

- [x] `av1/bitio`: `f(n)`, `su(n)`, `uvlc`, `leb128` reader
- [x] `av1/obu`: OBU header parsing, sequence header
- [x] `av1/obu`: frame header (intra-only path)
- [x] `av1/entropy`: symbol decoder (boolean coder) infrastructure
- [ ] `av1/entropy`: default CDF tables (hundreds of contexts from spec)
- [ ] `av1/predict`: intra modes — DC done; smooth / directional / Paeth / CFL pending
- [ ] `av1/transform`: IDCT4 done; IDCT8/16/32/64, ADST, FLIPADST, IDTX, WHT pending
- [ ] `av1/decoder`: package skeleton landed — parses OBUs through frame header,
      returns `ErrPixelDecodeUnimplemented` for the pixel path
- [ ] Partition tree decode + tile-group driver
- [ ] Dequant + reconstruction pipeline
- [ ] Deblocking loop filter
- [ ] CDEF (constrained directional enhancement filter)
- [ ] Top-level `goavif.Decode` → `image.Image` for 8-bit 4:2:0 still AVIF
- [ ] Golden tests vs dav1d on AOM intra-only test vectors

## Phase 3 — Full AV1 decoder

Everything the AV1 spec requires to pass the reference conformance suite.

- [ ] Inter prediction: single and compound references, translational and
      warp, global motion
- [ ] Loop restoration: Wiener + self-guided
- [ ] Film grain synthesis
- [ ] Segmentation + ref-frame management
- [ ] 10/12-bit pixel pipeline
- [ ] Monochrome, 4:2:2, 4:4:4 chroma
- [ ] Conformance harness driven from official AV1 test vectors

## Phase 4 — Alpha, HDR, image sequences

- [ ] Alpha: decode auxiliary AV1 stream, composite into `NRGBA`/`NRGBA64`
- [ ] HDR: surface CICP / transfer / matrix / range; honor 10/12-bit output
- [ ] Image sequences: parse `moov`/`trak`/`stbl` and expose per-frame timing
- [ ] `goavif.DecodeAll` public API

## Phase 5 — AV1 encoder: intra-only baseline

Mirror of the decoder plus minimal rate-distortion optimization.

- [ ] Forward transforms, forward quant
- [ ] Boolean writer + CDF-cost tables for RDO
- [ ] OBU writer (sequence header, frame header, tile group)
- [ ] Intra-only mode decision (Lagrangian RDO)
- [ ] Fixed-QP control, 8-bit 4:2:0
- [ ] Loop filter / CDEF parameter selection
- [ ] `goavif.Encode` producing AVIF stills that decode in dav1d / libavif

## Phase 6 — Full AV1 encoder

- [ ] Motion estimation + sub-pel refinement
- [ ] Partition/transform search
- [ ] Rate control (CBR / VBR / constant quality)
- [ ] Encoder support for alpha, 10/12-bit, image sequences
- [ ] Optional film-grain estimation

## Phase 7 — Performance

- [ ] Profile hot paths
- [ ] SIMD asm (`*_amd64.s`, `*_arm64.s`) under build tags without API change
- [ ] Parallel tile decode/encode

## Non-goals (for now)

- GPU / hardware acceleration
- Streaming / progressive decode
- AVIF grid / derived images
- Encryption / DRM
