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
- [x] CDEF: per-superblock cdef_idx signaling — ReadLiteral in the
      entropy decoder, per-SB idx stored in FrameState.CdefIdx, and
      cdef.ApplyFramePerSB routing the right (pri, sec) strength per
      64×64 SB (simplified: reads at SB close rather than first non-
      skip leaf — spec-exact ordering is a follow-up)
- [x] Loop restoration: Wiener 7-tap separable + SGR (dual-pass box +
      variance-adaptive a/b blend) primitives in av1/lr
- [ ] Loop restoration: per-unit signaling + frame driver wiring
      (tie into the decoder after CDEF)
- [x] Film grain synthesis: seeded LFSR RNG (spec §7.20.2) + piecewise-
      linear scaling-curve LUT + per-plane Apply driver (naive tiling)
- [x] Film grain: AR-coefficient shaping (spec §7.20.3.3) + grain
      template generator in av1/filmgrain/ar.go
- [x] Film grain: 73×73 luma + 38×38 chroma templates, 32×32 patch
      tiling via ApplyWithTemplate (av1/filmgrain/patch.go)
- [x] Film grain: film_grain_params wired from the frame header —
      per-plane scaling LUT + AR-shaped template + tiled apply as the
      final post-processing step in runTileGroup
- [ ] Film grain: spec-exact per-block hashing for the template
      offset (tile-structure-dependent, currently simplified)

### Top level
- [x] `colorspace`: YUV→RGB BT.601/709/2020 + Studio/Full range
- [x] `goavif.Decode`: container → av1C seq → primary item → tile
      decoder → deblock → CDEF → Frame → image.YCbCr
- [x] `goavif.DecodeConfig`: works standalone (ispe / pixi / av1C)
- [x] `cmd/goavif-info`: container/sequence-header inspector CLI
- [ ] Golden tests vs dav1d on AOM intra-only test vectors
- [x] 10/12-bit intra predictors: full uint16 coverage in
      av1/predict — DC/V/H/Paeth/Smooth/SmoothV/SmoothH
      (intra16.go), D45/D67/D113/D135/D157/D203 directional
      (intra16_dr.go), CFL subsample + pred (intra_cfl.go).
- [x] 10/12-bit reconstruction: Reconstruct16Block
      (av1/decoder/reconstruct.go) adds residual to uint16 pred and
      clips to (1<<bitDepth)-1.
- [x] 10/12-bit PredictIntra16 dispatch with Neighbors16 + half-range
      edge fallback (av1/decoder/predict_dispatch.go).
- [x] 10/12-bit plane storage: FrameState gains Y16 / U16 / V16
      uint16 buffers + BitDepth field. NewFrameStateHBD allocates
      uint16 planes; the 8-bit path is unchanged.
- [x] 10/12-bit CDEF: FilterBlock16 + FindDirection16 + Plane16 +
      ApplyFrame16 / ApplyFramePerSB16 mirror the uint8 drivers
      (av1/cdef/filter16.go, direction16.go).
- [x] 10/12-bit loop restoration: ApplyWiener16 + ApplySGR16 +
      Plane16 + ApplyFrame16 (av1/lr/wiener16.go, sgr16.go,
      frame16.go). SGR widens accumulators to int64 and scales p
      down by 2*(bd-8) so the x/(x+1) LUT index stays 8-bit.
- [x] 10/12-bit deblocking: Thresholds16 + NarrowMask16 +
      Filter4_16 + ApplyFrameNarrow16 (av1/loopfilter/narrow16.go,
      frame16.go). Clip bounds widen with bit depth; +4/+3
      rounding stays 8-bit per libaom's highbd_filter4.
- [x] 10/12-bit filter dispatch: applyLoopFilter / applyCDEF /
      applyLoopRestoration / applyFilmGrain all branch on
      fs.BitDepth and route to the uint16 primitives when >8.
      HBD pipeline integration test covers 10- and 12-bit at both
      luma and chroma.
- [x] 10/12-bit TileDecoder decode path: decodeLumaBlock16 +
      reconstructResidual16 + decodeChromaBlock16 +
      reconstructChromaResidual16 branch on fs.BitDepth > 8. CFL16
      subsamples from fs.Y16 and writes into fs.U16 / fs.V16.
      End-to-end HBD pipeline integration test runs partition walk
      + every post-processing step at 10 and 12 bit.
- [x] 10/12-bit public output: decoder.Frame gains Y16 / U16 / V16
      alongside the uint8 planes. goavif.Decode routes to
      NewFrameStateHBD for 10/12-bit sequences. frameToImage returns
      image.RGBA64 for 4:2:0 HBD (via colorspace.ConvertPlanar420_16)
      and image.Gray16 for monochrome HBD.
- [x] 10/12-bit film grain: filmgrain.ApplyWithTemplate16 tiles
      32×32 patches with bit-depth-aware clipping and restricted-
      range bounds scaled per bit depth (av1/filmgrain/apply16.go).
- [x] 10/12-bit colorspace: colorspace.YUVToRGB16 +
      ConvertPlanar420_16 produce 16-bit-per-channel output from
      10/12-bit YUV, honoring studio vs full range and CICP matrix
      selection (colorspace/yuv_rgb16.go).

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

- [x] Alpha: findAlphaItemID resolves auxl iref + auxC alpha URN,
      decodeAlphaFrame runs the aux AV1 stream through the existing
      intra-only path, compositeNRGBA / compositeNRGBA64 splice the
      alpha plane into image.NRGBA or image.NRGBA64 output. Decode
      detects the alpha item automatically.
- [x] HDR: CICP matrix + color range honored in the YUV→RGB matrix
      selection (frameToImage16 + composite helpers). 10/12-bit
      output lands as image.RGBA64 / image.NRGBA64. Transfer
      function passthrough is bitstream-level (decoder produces
      linear-relative YUV regardless of signaled transfer).
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
