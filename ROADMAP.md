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
- [x] `av1/entropy/cdfs`: CDF type + AomCDF inversion + full default tables:
      skip, partition (20 ctxs), kf_y_mode (5×5 ctxs), uv_mode (2×13),
      angle_delta (8), tx_size (4×3), txfm_partition (21), dc_sign (2×3),
      txb_skip (5×13, Q=0), eob_multi 16/32/64/128/256/512/1024 (all Q),
      eob_extra (4×5×2×9), coeff_base_multi (4×5×2×42), coeff_br_multi
      (4×5×2×21), coeff_base_eob_multi (Q=0), cfl_sign + cfl_alpha (6
      ctxs), spatial_pred_seg_tree (3 ctxs), nz_map_ctx_offset for all
      square + 8 rectangular TX shapes
- [ ] `av1/entropy/cdfs`: Q contexts 1-3 for txb_skip & coeff_base_eob,
      filter_intra, palette, intra_tx_type signaling
- [x] `av1/predict`: DC, V, H, Paeth, Smooth/V/H, full directional
      (D45/D67/D113/D135/D157/D203 via DirectionalPred), CFL scaffold
- [ ] `av1/predict`: angle_delta sub-pixel refinement, filter-intra,
      recursive intra, proper CFL (luma AC × signed alpha)
- [x] `av1/transform`: every 1D inverse transform AV1 defines:
      IDCT4/8/16/32/64, IADST4/8/16, IFLIPADST4/8/16, IDTX4/8/16/32,
      IWHT4 (lossless), FDCT4 (encoder skeleton), Inverse2D wrapper,
      RowOp/ColOp dispatch (4/8/16/32/64-point), DefaultZigzagScan
- [x] `av1/quant`: 8-bit DC/AC lookup tables + Params.Compute per plane
- [ ] `av1/quant`: 10/12-bit tables, Q-matrix / segment-Q application
- [x] `av1/decoder`: full pipeline — container → seq header → frame
      header → TileDecoder + CoeffDecoder → partition tree → per-leaf
      mode decode → intra predict → coefficient decode + dequant +
      inverse transform + reconstruct → loop filter → FrameState.Y/U/V
- [x] `av1/decoder`: CoeffDecoder reads txb_skip / eob / coeff_base /
      coeff_br / dc_sign / AC signs; sig_coef and level context
      derivation landed for 4×4, 8×8, 16×16
- [x] `av1/decoder`: intra_tx_type signaling wired (ReadIntraTxType +
      IntraTxTypeFor mapping for the 2 EXT_TX_SET_INTRA families)
- [ ] `av1/decoder`: TX_64x64 tile-level (needs top-left 32×32 subregion
      coeff layout), multi-tile frames, segment_id decoding, filter_intra
- [x] Reconstruction: per-block ReconstructBlock done
- [x] Loop filter: 4-tap narrow + 8-tap wide + frame-level driver;
      DeriveThresholds from filter_level / sharpness; Y+UV pass wired
      into the decoder after superblock loop
- [ ] Loop filter: 14-tap widest, per-edge TX-grid tracking (vs uniform
      8-pixel stride)
- [ ] CDEF (constrained directional enhancement filter)
- [x] `colorspace`: YUV→RGB BT.601/709/2020 + Studio/Full range
- [x] `goavif.Decode`: end-to-end pipeline wired (returns
      `ErrPixelDecodeUnimplemented` until coefficient decoding lands);
      Frame → image.Image bridge in place
- [x] `cmd/goavif-info`: container/sequence-header inspector CLI
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
