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

## Phase 4 — Alpha, HDR, image sequences ✅

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
- [x] Image sequences: parse `moov`/`trak`/`mdia`/`minf`/`stbl`
      plus stts/stsc/stsz/stco/co64/stss. Stbl.SampleTable walks the
      compact tables into a flat []Sample with per-sample IsSync
      flags. Moov.ImageTrackStbl dives through the trak hierarchy
      to find the image track.
- [x] `goavif.DecodeAll` public API: reads AVIF stills as a single
      1-frame slice, AVIS sequences as per-sample frames +
      time.Duration timings.
- [x] Sync-sample / CRA handling: stss parsing + per-sample IsSync
      in the sample table. DecodeAll decodes sync samples directly
      and repeats the previous frame for inter frames, returning
      ErrInterPredictionNotImplemented so callers can detect the
      degraded output. Full-inter support lands with Phase 5+.

## Phase 5 — AV1 encoder: intra-only baseline ✅

Mirror of the decoder plus minimal rate-distortion optimization.
This phase ships an end-to-end Encode → Decode round-trip using
our own codec. Output decodes cleanly through goavif.Decode and
produces a valid AVIF container; bit-exact interop with dav1d /
libavif would require minor spec-conformance tweaks but the
structural frame is complete.

- [x] Forward transforms: FDCT4/8/16/32/64 via `fdctMatrixInverse`
      (extracts AV1's inverse matrix Mᵢ by running IDCT on scaled
      basis vectors, applies Mᵢᵀ·y/(N·2048)). FIdentity4/8/16/32
      left-shift-by-1 IDTX. Round-trip tests verify ≤ N ulps error
      per coefficient.
- [x] Forward quantizer: `quant.QuantizeCoeff` / `QuantizeBlock`
      apply round(raw/q) with signed rounding, inverse of the
      existing dequantizer.
- [x] bitio writer: `bitio.Writer` mirrors the reader — F(n), Su(n),
      Uvlc, Leb128, Ns, TrailingBits, ByteAlign — with round-trip
      tests covering every read path.
- [x] Range encoder: `entropy.Encoder` uses deferred emission via
      math/big.Int — low accumulates across encodes at arbitrary
      precision, Finish() emits the initial 15 bits (XOR 0x7FFF of
      high-15-bits of low) plus `shift` renormalize bits. Bool,
      literal, and symbol (including implicit-last-symbol) round-
      trip with the decoder on long bursty sequences.
- [x] OBU writers: `obu.WriteSequenceHeader(w, h)` for
      reduced_still_picture_header mode; `obu.WriteKeyFrameHeader(w,
      h, baseQ)` emits uncompressed header with size-aware
      tile_info and CodedLossless shortcut when baseQ == 0;
      `obu.WrapOBU` adds OBU header + leb128 size.
- [x] Tile group writer: `av1/encoder.WriteIntraOnlyTile` emits
      a minimal payload — PARTITION_NONE + DC_PRED + skip=1 per
      superblock — exercising the new range encoder end-to-end
      against the decoder's CDF-driven partition / mode / skip /
      uv_mode reads.
- [x] `goavif.Encode` assembles sequence header OBU + frame OBU
      (frame header + tile group) + AVIF ISOBMFF container via
      BuildStillImage. Produces output that `goavif.Decode` reads
      back without error at 64×64 minimum. Pixel output is
      constant mid-grey (skip-only encoder); residual coefficient
      coding + real mode decision are the remaining work for a
      competitive encoder.

Follow-ups (outside the "baseline" goal of this phase):
- CDF-cost tables for RDO
- Intra-only mode decision (Lagrangian RDO)
- Coefficient write path (tx_type + base + br + sign emission)
- Loop filter / CDEF parameter selection
- Bit-exact spec conformance for dav1d/libavif interop

## Phase 6 — Full AV1 encoder

- [x] DC coefficient encoding: WriteCoefficients emits the full
      txb_skip / eob_pt / base_level / BR / sign symbol sequence,
      mirroring the decoder's ReadCoefficients. Luma and chroma
      planes both emit quantized DC residuals against DC_PRED.
      imageToYUV420 converts input images to BT.601 Y/Cb/Cr planes
      with 2×2 box subsampling. Decoded center-Y for a white image
      goes from 128 (old skip-only) to ~200 (now tracks input).
- [x] Full-spectrum coefficient encoding (multi-position AC):
      transform.Forward2D (DctDct + IDTX) covers 4/8/16/32/64-point
      row + column passes with round-trip parity vs Inverse2D.
      encoder.WriteIntraOnlyTile computes the 2D spatial residual
      against reconstructed-neighbor DC_PRED, forward-transforms at
      the block's TX size, zeros out positions outside the clamped
      scan region (TX_64×*), quantizes all coefficients, and emits
      them via WriteCoefficients. The encoder maintains its own
      reconstructed luma / chroma buffers so neighbor samples for
      the next block match what the decoder will reconstruct.
      Gradient + two-halves round-trip tests confirm AC harmonics
      reach the decoder. Fixed two bugs along the way:
      (1) decoder's ReadEOB elided the eob_extra low bits (replaced
      a fixed bias with real DecodeBool(16384) reads);
      (2) encoder's writeEOB swapped the EOBPtToEOB return values,
      treating extraBits as binStart;
      (3) decoder's runTileGroup mixed 8-pixel tile_info MI units
      with a 4-pixel multiplier, cutting every multi-SB frame's
      decode region in half.
- [x] Encoder support for alpha: opts.Alpha (or an input image with
      non-opaque pixels) triggers a second monochrome AV1 auxiliary
      item. obu.WriteMonoSequenceHeader + WriteMonoKeyFrameHeader emit
      the single-plane variants; isobmff.BuildStillImage adds the
      second infe + iloc extent, and registers an auxC box with the
      alpha URN plus an auxl iref from the alpha item to the primary.
      Gradient alpha round-trips: source 0..255 decodes to 10..234 at
      quality=90. Fixed one bug: WriteKeyFrameHeader's
      writeQuantParams / writeLoopFilterParams unconditionally emitted
      chroma delta flags — the parser's monochrome branch skips them,
      so the alpha frame header's bit-count went out of sync and the
      trailing bit check failed.
- [x] PARTITION_SPLIT at 64×64 → four 32×32 leaf blocks. The previous
      encoder used PARTITION_NONE at 64×64 → TX_64×64 with a clamped
      scan that dropped every coefficient outside the top-left 32×32
      subregion, flattening any high-frequency detail. Splitting
      yields four 32×32 TX blocks with full default-scan coefficient
      coverage. writeIntraTxTypeIfNeeded emits the required ext_tx
      symbol (DCT_DCT = raw 0) for TX sizes ≤ 32×32 per
      spec §6.10.15. A fine 64×64 checkerboard round-trips with row
      variance ≈ 10325 at quality=95 (near the theoretical maximum),
      where the previous encoder collapsed it to a uniform grey.
- [x] Intra mode search: encoder tracks mode info per 4×4 MI cell,
      computes above/left buckets (modeBucket mirroring the decoder),
      and for each 32×32 block picks the lowest-SAD mode among
      DC / V / H / Paeth / Smooth / SmoothV / SmoothH (directional
      modes still TODO — need extended-neighbor plumbing). Y mode is
      emitted via the context-dependent kfYModeCDF and UV mode via
      uvModeCDF[1][yMode], matching what the decoder reads.
- [x] Adaptive 32→16 partition split via highDetail32: when a 32×32
      block's best-mode SAD after the intra-mode search yields a mean
      absolute deviation > 20, the encoder emits PARTITION_SPLIT at
      the 32×32 level and codes four 16×16 leaves. Each 16×16 block
      uses TX_16×16 with txSet=1 (7 types) for tx_type signaling. For
      flat or gently graded regions we stay at 32×32 (no regression);
      for directional-bar and complex-texture content the finer
      partition cleanly preserves structure — 16×16 vertical bars
      round-trip to 55/186 vs source 59/188 (was 95/77 at 32×32).
- [x] Monochrome (grayscale) primary encoding. When the input is
      image.Gray or image.Gray16, goavif.Encode routes through the
      mono sequence/frame headers (already built for alpha) and
      passes nil chroma to WriteIntraOnlyTile. The container ends up
      with monochrome=1 in av1C and a 1-channel pixi; no chroma
      bitstream is coded. Gray gradient round-trips 0..255 → 20..233
      at quality=90 (matching the luma path of the color encoder).
- [x] AVIF grid image encode. goavif.EncodeGrid takes a slice of
      equal-dimension tile images + rows/cols + output dimensions
      and writes an AVIF container whose primary item is a
      "grid"-type derived image. isobmff.BuildGrid wires up the
      infe/iloc/iref/ipma plumbing: grid item with the 6/10-byte
      payload, one av01 item per tile (marked hidden), dimg iref
      listing tiles in raster order, shared av1C/pixi across tiles.
      2×2 round-trip preserves per-quadrant shade within quantizer
      drift (50/100/150/200 → 55/99/143/186 at q=90).
- [x] clap (clean-aperture) crop transform on decode. The primary
      item's Clap property is evaluated against the coded image:
      crop width / height / horizontal-offset / vertical-offset
      rationals compute a centered rectangle that becomes the
      returned sub-image. Applied between ispe cropping and
      irot/imir per HEIF §6.5.10's canonical order.
- [x] AVIF grid image decode. Primary items with ItemType "grid"
      now route through decodeGridPrimary: ParseImageGrid decodes
      the 6/10-byte grid payload (version, flags, rows_minus_one,
      columns_minus_one, output_width/height as 16 or 32 bits per
      flags bit 0); FindDimgTargets walks the `dimg` iref to
      enumerate tile item IDs; each tile is decoded through the
      single-item path and pasted into an output canvas clipped to
      output_width × output_height. This is the format Apple uses
      for iPhone HEIC/AVIF photos. ispe cropping and irot/imir
      transforms still apply via the shared post-transform pass.
- [x] irot / imir transform application on decode. Decode walks iprp
      for Irot / Imir properties associated with the primary item
      and applies them to the decoded image: Irot.Angle (0..3) picks
      how many 90° CCW rotations to apply; Imir.Axis selects
      horizontal (Axis=1) vs vertical (Axis=0) flip, applied in
      iprp association order. Helpers rotate90CCW / mirror verified
      by unit tests.
- [x] Directional intra with extended neighbors. decodeLeafBlock /
      decodeLumaBlock16 now build bw+bh-length above/left arrays
      alongside the shorter bw/bh ones, with edge-extension when the
      frame boundary is reached. encoder.buildNeighbors /
      buildNeighbors16 mirror the setup. Neighbors / Neighbors16's
      AboveExtended / LeftExtended fields are populated on every
      block so predict.DirectionalPred and its uint16 counterpart
      can project angles across the full block without clamping at
      the block edge. Encoder's intra-mode search adds D45 / D67 /
      D113 / D135 / D157 / D203 candidates when both neighbors are
      available. Round-trip tests pass with only minor variance
      change on a synthetic checkerboard (10589 → 9896 at q=95).
- [x] 4:2:2 and 4:4:4 chroma encoding + HBD chroma generalization.
      obu.WriteSequenceHeaderFull emits sequence headers at profile 0
      (4:2:0), profile 1 (4:4:4), or profile 2 (4:2:2 / 12-bit),
      picking the right profile based on the bit depth + subsampling
      pair. Encoder tile threads subX/subY through writeChromaDCLeaf
      / writeChromaSkipReconstruction so the chroma block size scales
      correctly. goavif.Encode picks subsampling from image.YCbCr's
      native SubsampleRatio (when present) or opts.ChromaSubsampling
      (override), defaulting to 4:2:0. colorspace.ConvertPlanar16
      generalizes the HBD YUV→RGB converter to arbitrary subsampling
      so Decode returns proper RGBA64 for 10-bit 4:2:2 / 4:4:4.
      Round-trips pass for 4:2:0 / 4:2:2 / 4:4:4 at both 8-bit and
      10-bit; chroma profile switches verified via ParseSequenceHeader.
- [x] Token qCtx threading. The encoder previously hard-coded qCtx=0
      in every coefficient CDF lookup (coeff_base_multi, coeff_br_multi,
      eob_multi, eob_extra). The decoder derives qCtx from
      base_q_index per spec §7.12.4 — for baseQ ≥ 64 this diverges
      from 0. Added qCtx to encState (via qIndexToCtx mirroring
      decoder) and threaded it through WriteCoefficients and
      writeEOB. Low-quality settings (q=10 → baseQ≈230 → qCtx=3)
      now decode correctly.
- [x] Arbitrary-dimension support. goavif.Encode auto-pads frames
      that aren't multiples of 64 by edge-extending the right/bottom
      border; the coded frame carries the padded dimensions while
      ispe records the caller-visible size. goavif.Decode looks up
      ispe on the primary item and crops the coded frame to that
      rectangle via image.Image.SubImage, so callers get the exact
      dimensions they passed to Encode. Round-trips verified at
      100×100, 133×133, 200×200.
- [x] Intra-only AVIS sequence encoding. goavif.EncodeAll takes a
      slice of images + per-frame delays and writes an AVIS container
      (ftyp brand "avis", compatible_brands include "avif", "msf1",
      "miaf"). Each frame is encoded as a self-contained AV1 stream
      (seq-header OBU + frame OBU + tile group) so every sample is a
      sync point. MarshalPayload implementations added for Mvhd,
      Stts, Stsc, Stsz, Stco, Co64, Stss; tkhd/mdhd/vmhd/dinf/stsd/
      av01 sample entries are built as RawBox with hand-constructed
      payloads. isobmff.BuildSequence assembles ftyp+meta+moov+mdat,
      computes mdat offsets, and patches stco + iloc after layout.
      3-frame round-trip preserves per-frame luma shade (50/120/190
      decodes to 55/125/186) and durations (within 10 ms).
- [x] 10-bit (HBD) encoder path. WriteSequenceHeaderHBD and
      WriteMonoSequenceHeaderHBD emit profile-0 high_bitdepth=1
      sequence headers. New av1/encoder/tile16.go mirrors
      WriteIntraOnlyTile / writeLeaf / chooseIntraMode /
      reconstructAndWrite for uint16 sample buffers; the shared
      symbol emission (partition / modes / tx_type / coefficients)
      is reused unchanged. goavif.Encode auto-selects the HBD path
      for image.NRGBA64 / RGBA64 / Gray16 inputs (or explicit
      opts.BitDepth), extracts BT.601 studio-range uint16 Y/U/V via
      imageToYUV420_16 / imageToLuma16, and runs the HBD tile
      writer. The container's av1C, pixi, and colr already honour
      BitDepth. Alpha items go HBD too when the primary is HBD.
      A 10-bit NRGBA64 gradient round-trips to a width-equivalent
      NRGBA-shifted range (12..245 in 8-bit terms at quality=90).
      12-bit is also wired up: WriteSequenceHeaderHBD selects
      seq_profile=2 + twelve_bit=1 + explicit 4:2:0 subsampling bits,
      and opts.BitDepth=12 routes through. A 12-bit NRGBA64 gradient
      (0..4095) round-trips to 12..243 in 8-bit terms at quality=90.
- [x] Golomb-rice tail for coefficient levels > 15. The previous
      encoder and decoder both capped at base+BR = 15 (base 3 +
      4 × BR ≤ 3), silently truncating any larger quantized
      magnitude. A 64×64 sharp step (luma 30 → 220) at quality=98
      now round-trips to left=44 / right=206, near the source
      BT.601 luma of 44 / 209; previously it would have flattened.
      Added writeGolomb / readGolomb helpers (uniform 50/50 bypass
      bits; length zeros + terminating 1 + length low bits of
      value+1).
- [x] Motion estimation (integer-pel): `encoder.SearchMV` does a full
      SAD search over a configurable window; `encoder.DiamondSearchMV`
      uses an 8-point diamond descent for faster ME on large windows.
      Both clamp the reference access to the frame edge (matching the
      decoder's MotionCompensate behavior) so MVs pointing past the
      frame are legal. `encoder.WriteInterMETile` runs ME per 32×32
      block and emits the best MV + residual — ready to slot in as
      the inter-frame encoder for AVIS sequences.
- [x] AVIS inter-frame encoding end-to-end: `Options.InterEnabled` +
      `Options.KeyFrameInterval` opt the AVIS encoder into real inter
      compression. Frame 0 is always a sync keyframe; subsequent
      frames within the keyframe interval use `WriteInterMETile`
      against the previously decoded frame. `SequenceFrame.IsSync`
      threads per-frame sync flags through the stss table builder.
      `DecodeAll` now routes each sample through `DecodeWithRef` so
      inter samples are reconstructed with motion compensation.
      Gradient round-trip across a 3-frame inter sequence recovers
      per-frame shade within ±10.
- [ ] Sub-pel motion estimation refinement (currently integer-pel only)
- [ ] Transform / mode / partition RDO search (currently hard-coded
      DC_PRED + SPLIT + DCT_DCT everywhere)
- [ ] Rate control (CBR / VBR / constant quality)
- [ ] Encoder support for 10/12-bit image sequences (inter path is 8-bit
      only today; 10/12-bit sequences fall back to intra-only keyframes)
- [ ] Optional film-grain estimation

## Phase 5 — Inter prediction (in progress)

Started as of this session. Decoder infrastructure and primitives
are landing; block-level syntax and MC integration are the remaining
work before inter frames actually decode to correct pixels.

- [x] Default inter CDFs ported from libaom (entropymode.c /
      entropymv.c). Added to `av1/entropy/cdfs/inter.go`:
      is_inter / skip_mode / single_ref / newmv / zeromv / refmv /
      drl / mv_joint / mv_sign / mv_class / mv_class0_bit /
      mv_class0_fr / mv_class0_hp / mv_fr / mv_hp / mv_bits /
      interp_filter / y_mode (inter-frame variant, not kf).
- [x] MV decoder: `decoder.MVDecoder` reads mv_joint + per-component
      sign / class / class0-bit / class0-fr / class0-hp / fr / hp /
      bits per spec §6.10.27. Tests cover zero joint, positive
      class-0, and negative class-1 cases — all round-trip through
      the entropy encoder.
- [x] 8-tap sub-pel interpolation filters: REGULAR / SMOOTH / SHARP
      at 16 phases × 8 taps each. `predict.InterpSubPel` runs
      horizontal-then-vertical pass with int32 accumulator and
      (1<<14) rounding; `predict.InterpInteger` is the zero-phase
      fast path. Tests verify zero-phase pass-through and half-pel
      shift interpolates between neighboring integer samples.
- [x] Reference frame buffer: `FrameState.RefFrame` carries the
      previously decoded frame so inter blocks can source MC samples.
      TileDecoder gains `.inter` / `.refY` / `.refU` / `.refV` slots.
- [x] Motion compensation: `decoder.MotionCompensate` fetches from
      a reference plane at an eighth-pel MV offset with edge clamp;
      integer-pel fast path + 8-tap sub-pel slow path. 3 tests pass.
- [x] Block-level inter syntax reader: `decoder.InterDecoder` wraps
      the 18 inter CDFs and exposes ReadIsInter / ReadSingleRefFrame
      / ReadInterMode / ReadMV / ReadInterpFilter / ReadYMode /
      ReadSkip — the full set of symbol readers a single-ref NEWMV
      inter block needs. Rejects compound / multi-ref branches with
      ErrUnsupportedInterMode.
- [x] Decoder integration: `decodeLeafBlock` dispatches through
      `decodeInterLeafBlock` when the tile decoder holds an inter
      reader. `decodeInterLeafBlock` runs is_inter context, NEWMV
      MV decode, motion compensation, residual add, and chroma MC
      with subsampled MV scaling. Intra blocks within inter frames
      route through the inter-frame Y-mode CDF.
- [x] Public API: `decoder.DecodeWithRef(data, seq, ref *Frame)`
      and `NewTileDecoderWithRef` accept the previously decoded
      frame as MC source. `DecodeAll` threads each decoded frame
      as the ref for the next sample. `TestDecodeWithRefIntraEquivalent`
      verifies the new entry point round-trips intra content
      identically to `Decode`.

- [x] End-to-end inter prediction: `obu.WriteSequenceHeaderAVIS` emits
      a non-reduced-still-picture-header sequence so inter frames are
      legal; `obu.WriteAVISKeyFrameHeader` + `obu.WriteInterFrameHeader`
      pair with it for AVIS keyframes and inter frames. The inter
      frame-header parser handles the inter branch (ref_frame_idx,
      interpolation_filter, is_motion_mode_switchable, use_ref_frame_mvs).
      `encoder.WriteInterCopyTile` produces a structurally valid inter
      tile where every block is single-ref LAST / NEWMV / zero-MV /
      skip_txfm — a degenerate "copy frame" that should decode to the
      reference pixels exactly. Generalized via
      `encoder.WriteInterUniformMVTile` (uniform non-zero MV) and
      `encoder.WriteInterResidualTile` (MV + quantized Y/UV residual
      against MC prediction). Four round-trip tests validate the
      loop: copy, horizontal MV, vertical MV, residual (MAD ≈ 4
      samples at baseQ=40). Encoder's `writeMV` / `writeMVComponent`
      mirror the decoder's MV reader so mv_joint / sign / class /
      bits all round-trip. Encoder/decoder track is_inter neighbor
      context symmetrically.
- [ ] Compound prediction, global motion, warped motion, OBMC,
      inter-intra, ref MV list construction for NEAREST/NEAR/GLOBAL
      modes, non-zero MV coding, residual-bearing inter blocks — the
      primitives (MV decoder supports full syntax, InterpSubPel
      supports any phase) exist; these are follow-up extensions that
      build on the working pipeline rather than new infrastructure.

## Phase 7 — Performance

- [x] First-pass allocation cleanup:
      - fdctMatrixInverse used to build the IDCT matrix from basis
        vectors on every call (N+1 slice allocs per call). Now cached
        globally per size in transform.miFor, backed by sync.Once.
      - Encoder wrapped every symbol's CDF in
        `append(cdfs.CDF(nil), ...)` before calling EncodeSymbol — ~20
        copies per leaf block. Our encoder runs with updateCDF=false
        (disable_cdf_update=1), so EncodeSymbol doesn't mutate the
        CDF; defaults are now passed by reference.
      - entropy.Encoder allocated a fresh big.Int per EncodeBool /
        EncodeSymbol for the loAdd / split value. Added a scratch
        big.Int on the encoder and SetUint64 + Add in place.
      - imageToYUV420 / imageToAlpha / wantAlpha had per-pixel
        m.At(x, y).RGBA() calls that box a Color in an interface;
        added fast paths for *image.RGBA / *image.NRGBA / *image.YCbCr
        that read src.Pix directly.
      - Net: 64×64 encode ~27300 allocs → 502 allocs (~54× less),
        256×256 encode ~415000 allocs → 3412 allocs (~121× less).
        Wall time roughly halved.
- [ ] SIMD asm (`*_amd64.s`, `*_arm64.s`) under build tags without API change
- [ ] Parallel tile decode/encode

## Non-goals (for now)

- GPU / hardware acceleration
- Streaming / progressive decode
- AVIF grid / derived images
- Encryption / DRM
