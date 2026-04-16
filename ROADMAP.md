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

## Phase 2 — AV1 intra decoder (feature-complete for the common path) ✅

End-to-end decode of an AVIF still: container → AV1 bitstream → Y/U/V
planes → image.Image. The pipeline runs bitstream-accurately through
CDF-driven partition / mode / coefficient / sign / tx_type decode for
TX sizes up to 32×32 (square) and up to 32×8 / 8×32 (non-square) with
the full intra mode set (DC/V/H/Smooth*/Paeth/D45-D67) and proper CFL.

### Foundational packages
- [x] `av1/bitio`: `f(n)`, `su(n)`, `uvlc`, `leb128`, ns/trailing bits
- [x] `av1/obu`: OBU framing + sequence header + full intra frame header
- [x] `av1/entropy`: symbol decoder (range-coded CDF) with update
- [x] `av1/entropy/cdfs`: every mode-level + coefficient-level default
      CDF the spec's intra path consumes — partition, kf_y_mode, uv_mode,
      angle_delta, tx_size, txfm_partition, skip, dc_sign, txb_skip,
      eob_multi 16/32/64/128/256/512/1024, eob_extra, coeff_base_multi
      (42 sig contexts), coeff_br_multi (21 level contexts),
      coeff_base_eob_multi, cfl_sign, cfl_alpha (6 contexts),
      spatial_pred_seg_tree, intra_ext_tx (sets 1 + 2),
      nz_map_ctx_offset (all square + 8 rectangular shapes)
- [ ] `av1/entropy/cdfs`: Q contexts 1-3 for txb_skip & coeff_base_eob,
      filter_intra, palette (deferred — uncommon in AVIF)

### Per-block primitives
- [x] `av1/predict`: DC, V, H, Paeth, Smooth/V/H, full directional
      (D45/D67/D113/D135/D157/D203 via `DirectionalPred` with sub-pixel
      interpolation from the spec's `dr_intra_derivative`), CFL
      (subsample + signed alpha)
- [ ] `av1/predict`: angle_delta refinement bits, filter-intra,
      recursive intra (deferred)
- [x] `av1/transform`: every 1D inverse transform AV1 defines —
      IDCT4/8/16/32/64, IADST4/8/16, IFLIPADST4/8/16, IDTX4/8/16/32,
      IWHT4 (lossless), FDCT4 (encoder skeleton), 2D Inverse2D wrapper,
      RowOp/ColOp dispatch covering every (TxType × TxSize) pair for
      4/8/16/32-point and 64-point DCT+IDTX, DefaultZigzagScan
- [x] `av1/quant`: 8/10/12-bit DC/AC lookup tables + per-plane Compute
      with segmentation Q offset application
- [ ] `av1/quant`: full Q-matrix application when using_qmatrix=1

### Tile / superblock decoder
- [x] `av1/decoder` core:
  - TileDecoder reads partition / intra Y mode / UV mode / angle_delta /
    skip / segment_id / intra_tx_type symbols from the bitstream
  - CoeffDecoder reads txb_skip / eob_pt + eob_extra / coeff_base_multi /
    coeff_br_multi / dc_sign / AC uniform signs, with sig_coef_ctx and
    level_ctx derivation per spec §6.10.6
  - Superblock partition tree — NONE / HORZ / VERT / SPLIT +
    HORZ_A/B / VERT_A/B / HORZ_4 / VERT_4 (all 10 partition types)
  - Per-block intra predict → dequant (base + segmentation Δ) → inverse
    2D transform (tx_type dispatched) → reconstruct → plane write
  - Chroma pipeline: UV mode decode, optional CFL with reconstructed
    luma AC + decoded alpha, per-plane coefficient decode + dequant
  - Multi-tile tile-group support (tile_size_minus_1 leb128 prefixes)
  - TX_64x64 / 64x32 / 32x64 via transform.ClampedScan (32×32 coded
    subregion per spec §7.7.3)
  - Edge-of-frame V/H/Paeth fallback to half-range samples
- [ ] `av1/decoder` stragglers:
  - filter_intra / palette / intrabc modes
  - Per-superblock cdef_idx signaling (uses strengths[0] as default)

### Filters and output
- [x] Loop filter: 4-tap narrow + 8-tap wide + frame-level driver;
      DeriveThresholds from filter_level / sharpness; Y+UV pass wired
      into the decoder after superblock loop
- [ ] Loop filter: 14-tap widest filter, per-edge TX-grid tracking (vs
      uniform 8-pixel stride)
- [x] CDEF: Constrain nonlinearity + primary/secondary FilterBlock +
      FindDirection from libaom's cdef_find_dir_c + ApplyFrame driver;
      wired into the decoder after deblocking when sh.EnableCdef
- [ ] CDEF: per-superblock cdef_idx signaling (currently uses
      strengths[0] as a sensible default)
- [x] Loop restoration: Wiener 7-tap separable filter primitives (av1/lr)
- [ ] Loop restoration: self-guided (SGR), per-unit signaling, frame driver
- [ ] Film grain synthesis

### Top level
- [x] `colorspace`: YUV→RGB BT.601/709/2020 + Studio/Full range
- [x] `goavif.Decode`: container → av1C seq → primary item → tile
      decoder → deblock → CDEF → Frame → image.YCbCr
- [x] `goavif.DecodeConfig`: works standalone (ispe / pixi / av1C)
- [x] `cmd/goavif-info`: container/sequence-header inspector CLI
- [ ] Golden tests vs dav1d on AOM intra-only test vectors
- [ ] 10/12-bit pixel plane path (quant tables in place; FrameState is
      still uint8 — needs uint16 plane type + colorspace widening)

## Phase 3 — spec-complete decoder

Parity with the reference conformance suite.

- [ ] Inter prediction: single and compound references, translational
      and warp, global motion
- [ ] Loop restoration (Wiener + self-guided): move from Phase 2
- [ ] Film grain synthesis: move from Phase 2
- [ ] Full ref-frame management + segmentation temporal update
- [ ] 10/12-bit pixel pipeline end-to-end
- [ ] Monochrome, 4:2:2, 4:4:4 chroma sampling fully exercised
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
