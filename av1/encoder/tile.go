// Package encoder assembles AV1 bitstreams for the goavif encoder. It
// pairs with [av1/decoder] at the syntax level: whatever encoder
// produces should be directly consumable by decoder.Decode.
//
// The encoder emits PARTITION_NONE + DC_PRED blocks. When a luma
// plane is provided, the encoder computes the 2D residual (source
// minus DC prediction), forward-transforms it, quantizes all
// coefficients, and emits them via [WriteCoefficients]. The encoder
// maintains its own reconstructed luma/chroma buffers so DC_PRED
// neighbor samples mirror what the decoder will build.
package encoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/obu"
	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// WriteIntraOnlyTile emits a tile payload for an intra-only keyframe
// of dimension (width, height). Every superblock is encoded as a
// single 64×64 PARTITION_NONE block with DC_PRED y-mode and DC_PRED
// uv-mode.
//
// When lumaY is non-nil (length w*h, row-major, stride=width), the
// encoder computes the 2D residual against DC_PRED for each block
// and emits quantized coefficients. chromaU/chromaV are (w/2)*(h/2)
// chroma planes; when non-nil, chroma coefficients are emitted too.
// When nil, the corresponding planes are skip.
func WriteIntraOnlyTile(width, height int, fh *obu.FrameHeader, sh *obu.SequenceHeader, lumaY, chromaU, chromaV []uint8) ([]byte, error) {
	if sh == nil || fh == nil {
		return nil, fmt.Errorf("encoder: nil sh / fh")
	}

	var enc entropy.Encoder
	enc.Init(!fh.DisableCDFUpdate)

	sbSize := 64
	if sh.Use128x128Superblock {
		sbSize = 128
	}
	baseQ := int(fh.Quant.BaseQIndex)
	subX := int(sh.Color.SubsamplingX)
	subY := int(sh.Color.SubsamplingY)

	cw := width >> subX
	if cw < 1 {
		cw = 1
	}
	ch := height >> subY
	if ch < 1 {
		ch = 1
	}

	// Reconstructed buffers for intra prediction neighbor lookup.
	recY := make([]uint8, width*height)
	recU := make([]uint8, cw*ch)
	recV := make([]uint8, cw*ch)

	// Mode tracking — per-4×4-MI slot, used to derive the Y mode CDF
	// context (aboveBucket / leftBucket) that the decoder will use.
	miCols := (width + 3) >> 2
	miRows := (height + 3) >> 2
	st := &encState{
		modes:  make([]decoder.IntraMode, miCols*miRows),
		miCols: miCols,
		miRows: miRows,
		qCtx:   qIndexToCtx(baseQ),
		subX:   subX,
		subY:   subY,
	}

	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			if err := writeSuperblock(&enc, x, y, sbSize, width, height, cw, ch, sh,
				lumaY, chromaU, chromaV,
				recY, recU, recV, st,
				baseQ); err != nil {
				return nil, err
			}
		}
	}
	return enc.Finish(), nil
}

// encState carries per-frame mutable state the encoder builds alongside
// the bitstream: the 4×4 MI mode grid (used for Y mode CDF context)
// and its dimensions. qCtx is the token-CDF quantizer bucket derived
// from base_q_index (spec §7.12.4) and must match the decoder's
// InitCoeffDecoder argument for coefficient round-trip.
type encState struct {
	modes  []decoder.IntraMode
	miCols int
	miRows int
	qCtx   int
	// subX / subY are the chroma subsampling factors (0 = full res,
	// 1 = half res on that axis). Populated from the sequence header
	// at WriteIntraOnlyTile entry; shared by the 8-bit and HBD tile
	// writers.
	subX int
	subY int
}

// qIndexToCtx mirrors decoder.qIndexToCtx: 0..63 → 0, 64..127 → 1,
// 128..191 → 2, 192..255 → 3.
func qIndexToCtx(q int) int {
	switch {
	case q < 64:
		return 0
	case q < 128:
		return 1
	case q < 192:
		return 2
	}
	return 3
}

// setMode stamps the chosen intra mode across every 4×4 MI cell a block
// covers. miX / miY are in MI units; miW / miH are cell counts.
func (s *encState) setMode(miX, miY, miW, miH int, m decoder.IntraMode) {
	for r := 0; r < miH && miY+r < s.miRows; r++ {
		for c := 0; c < miW && miX+c < s.miCols; c++ {
			s.modes[(miY+r)*s.miCols+(miX+c)] = m
		}
	}
}

// modeCtx returns (aboveBucket, leftBucket) for the Y mode CDF at the
// given block origin.
func (s *encState) modeCtx(miX, miY int) (int, int) {
	above := decoder.DCPred
	left := decoder.DCPred
	if miY > 0 && miY-1 < s.miRows && miX < s.miCols {
		above = s.modes[(miY-1)*s.miCols+miX]
	}
	if miX > 0 && miX-1 < s.miCols && miY < s.miRows {
		left = s.modes[miY*s.miCols+(miX-1)]
	}
	return modeBucket(above), modeBucket(left)
}

// modeBucket mirrors decoder.modeCtxBucket: DC=0, V=1, H=2, directional=3, others=4.
func modeBucket(m decoder.IntraMode) int {
	switch m {
	case decoder.DCPred:
		return 0
	case decoder.VPred:
		return 1
	case decoder.HPred:
		return 2
	case decoder.D45Pred, decoder.D135Pred, decoder.D113Pred,
		decoder.D157Pred, decoder.D203Pred, decoder.D67Pred:
		return 3
	}
	return 4 // SMOOTH, SMOOTH_V, SMOOTH_H, PAETH
}

// writeSuperblock emits the syntax for a single SB.
func writeSuperblock(enc *entropy.Encoder, x, y, sbSize, frameW, frameH, cw, ch int, sh *obu.SequenceHeader,
	lumaY, chromaU, chromaV []uint8,
	recY, recU, recV []uint8,
	st *encState,
	baseQ int) error {
	// When the SB is fully inside the frame, split into four 32×32
	// blocks. This avoids the TX_64×64 clamped scan (which drops
	// frequencies outside the top-left 32×32) and gets us per-quadrant
	// coefficient coding at 32×32.
	//
	// For SBs that extend past the frame edge, fall back to
	// PARTITION_NONE at the full SB size — the leaf block clips to the
	// visible region and the clamped scan handles the outer coefs.
	if sbSize == 64 && x+sbSize <= frameW && y+sbSize <= frameH {
		writePartitionSymbol(enc, 3 /* bsl=3 for 64×64 */, 0 /* ctx */, 3 /* PARTITION_SPLIT */)
		for _, off := range [4][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}} {
			qx := x + off[0]
			qy := y + off[1]
			// Before emitting a 32×32 leaf, check whether further
			// splitting into 16×16 blocks would materially reduce the
			// prediction error. highDetail32 runs the same mode search
			// the leaf would run and returns true when the best-mode
			// SAD exceeds a heuristic threshold.
			if lumaY != nil && highDetail32(lumaY, recY, qx, qy, frameW, frameH) {
				writePartitionSymbol(enc, 2 /* bsl=2 for 32×32 */, 0 /* ctx */, 3 /* PARTITION_SPLIT */)
				for _, off2 := range [4][2]int{{0, 0}, {16, 0}, {0, 16}, {16, 16}} {
					qqx := qx + off2[0]
					qqy := qy + off2[1]
					writePartitionSymbol(enc, 1 /* bsl=1 for 16×16 */, 0, 0 /* PARTITION_NONE */)
					writeLeaf(enc, sh, qqx, qqy, 16, 16, frameW, frameH, cw, ch,
						lumaY, chromaU, chromaV,
						recY, recU, recV, st,
						baseQ)
				}
				continue
			}
			writePartitionSymbol(enc, 2 /* bsl=2 for 32×32 */, 0 /* ctx */, 0 /* PARTITION_NONE */)
			writeLeaf(enc, sh, qx, qy, 32, 32, frameW, frameH, cw, ch,
				lumaY, chromaU, chromaV,
				recY, recU, recV, st,
				baseQ)
		}
		return nil
	}

	writePartitionSymbol(enc, partitionBsl(sbSize), 0, 0 /* PARTITION_NONE */)
	bw := sbSize
	bh := sbSize
	if x+bw > frameW {
		bw = frameW - x
	}
	if y+bh > frameH {
		bh = frameH - y
	}
	if bw <= 0 || bh <= 0 {
		return nil
	}
	writeLeaf(enc, sh, x, y, bw, bh, frameW, frameH, cw, ch,
		lumaY, chromaU, chromaV,
		recY, recU, recV, st,
		baseQ)
	return nil
}

// partitionBsl returns the BSL bucket used by the decoder's partition
// CDF lookup: 3 = 64x64, 4 = 128x128.
func partitionBsl(sbSize int) int {
	if sbSize == 128 {
		return 4
	}
	return 3
}

// writePartitionSymbol emits a partition symbol with the given BSL
// (block-size-log) and context, mirroring decoder.decodePartitionNode's
// CDF lookup of cdfIdx = bsl*4 + ctx.
func writePartitionSymbol(enc *entropy.Encoder, bsl, ctx, symbol int) {
	cdfIdx := bsl*4 + ctx
	if cdfIdx >= len(cdfs.DefaultPartitionCDF) {
		return
	}
	enc.EncodeSymbol(cdfs.DefaultPartitionCDF[cdfIdx], symbol)
}

// writeLeaf emits the mode + coefficient syntax for a single leaf
// block. For luma it runs a small intra-mode search (DC/V/H/Paeth/
// Smooth/SmoothV/SmoothH) and picks the mode with smallest SAD
// against the source. Chroma uses DC_PRED unconditionally (matching
// the decoder's simple path). The encoder's reconstructed buffers are
// updated after each block so later blocks see the correct neighbor
// samples.
func writeLeaf(enc *entropy.Encoder, sh *obu.SequenceHeader,
	bx, by, bw, bh, frameW, frameH, cStrideW, cStrideH int,
	lumaY, chromaU, chromaV []uint8,
	recY, recU, recV []uint8,
	st *encState,
	baseQ int) {
	miX := bx >> 2
	miY := by >> 2
	miW := bw >> 2
	miH := bh >> 2
	aboveBucket, leftBucket := st.modeCtx(miX, miY)

	// Run intra-mode search on the luma block (when source data is
	// present). Candidates are the 7 non-directional modes; SAD is
	// computed against the source block.
	chosenMode := decoder.DCPred
	var lumaPred []uint8
	if lumaY != nil && bw > 0 && bh > 0 {
		chosenMode, lumaPred = chooseIntraMode(lumaY, recY, bx, by, bw, bh, frameW, frameH)
	}

	// Emit chosen Y intra mode via the context-dependent kf_y_mode CDF.
	enc.EncodeSymbol(cdfs.DefaultKfYModeCDF[aboveBucket][leftBucket], int(chosenMode))
	// Record the chosen mode so neighboring blocks derive the correct
	// bucket when it's their turn.
	st.setMode(miX, miY, miW, miH, chosenMode)

	// Transform / quantize / coefficient build-up.
	var hasLumaResidual bool
	var lumaCoeffs []int32
	var lumaScan []int
	var lumaNzMap []int8
	var lumaTxSizeIdx int
	var lumaTxSize transform.TxSize
	var lumaTxW, lumaTxH int
	var lumaDequant []int32

	if lumaY != nil && bw > 0 && bh > 0 {
		lumaTxSizeIdx, lumaNzMap, lumaScan, lumaTxSize, lumaTxW, lumaTxH = selectEncTxParams(bw, bh)

		// Compute residual = source - prediction.
		residual := make([]int32, lumaTxW*lumaTxH)
		for r := 0; r < bh; r++ {
			srcRow := (by + r) * frameW
			predRow := r * bw
			for c := 0; c < bw; c++ {
				residual[r*lumaTxW+c] = int32(lumaY[srcRow+bx+c]) - int32(lumaPred[predRow+c])
			}
		}

		if err := transform.Forward2D(residual, transform.DctDct, lumaTxSize); err != nil {
			residual = nil
		}

		if residual != nil {
			enforceClampedScan(residual, lumaTxSize, lumaTxW, lumaTxH)

			qp := quant.Params{BaseQIndex: baseQ, BitDepth: 8}
			qv := qp.Compute(quant.PlaneY)
			lumaCoeffs = make([]int32, lumaTxW*lumaTxH)
			for i := range residual {
				lumaCoeffs[i] = quant.QuantizeCoeff(residual[i], i, qv)
			}

			for _, v := range lumaCoeffs {
				if v != 0 {
					hasLumaResidual = true
					break
				}
			}

			if hasLumaResidual {
				lumaDequant = make([]int32, lumaTxW*lumaTxH)
				for i, v := range lumaCoeffs {
					lumaDequant[i] = decoder.DequantCoeff(v, i, qv)
				}
			}
		}
	}

	// Skip flag.
	skipCDF := cdfs.DefaultSkipCDF[0]
	if hasLumaResidual {
		enc.EncodeSymbol(skipCDF, 0)
	} else {
		enc.EncodeSymbol(skipCDF, 1)
	}

	// UV mode: always DC_PRED for now. CDF depends on the chosen Y
	// mode.
	if !sh.Color.Monochrome {
		enc.EncodeSymbol(cdfs.DefaultUVModeCDF[1][int(chosenMode)], 0 /* DC_PRED */)
	}

	if !hasLumaResidual {
		if lumaPred != nil {
			writeBack(recY, lumaPred, bx, by, bw, bh, frameW)
		}
		if !sh.Color.Monochrome {
			writeChromaSkipReconstruction(recU, recV, bx, by, bw, bh,
				cStrideW, cStrideH, st.subX, st.subY)
		}
		return
	}

	// tx_type symbol for TX <= 32×32 (always DCT_DCT, raw 0).
	writeIntraTxTypeIfNeeded(enc, bw, bh, int(chosenMode))

	WriteCoefficients(enc, lumaCoeffs, lumaTxSizeIdx, 0 /*luma*/, st.qCtx, lumaScan, lumaNzMap, lumaTxW, lumaTxH)

	reconstructAndWrite(recY, lumaPred, lumaDequant,
		bx, by, bw, bh, lumaTxW, lumaTxH,
		transform.DctDct, lumaTxSize,
		frameW)

	if !sh.Color.Monochrome {
		writeChromaDCLeaf(enc, bx, by, bw, bh, cStrideW, cStrideH,
			chromaU, chromaV, recU, recV, baseQ, st.qCtx, st.subX, st.subY)
	}
}

// highDetail32 returns true when a 32×32 block has enough residual
// energy (after the encoder's intra-mode search) that splitting into
// four 16×16 sub-blocks is expected to reduce coded size / improve
// quality. The threshold is a heuristic — proper RDO is Phase 6+
// work; this gets most of the benefit for complex texture areas
// while avoiding the per-block cost of unconditional splitting.
//
// Returns false when source data is unavailable (falls through to
// the default 32×32 path) or when the block extends past the frame
// (can't be evenly split).
func highDetail32(lumaY, recY []uint8, bx, by, frameW, frameH int) bool {
	if bx+32 > frameW || by+32 > frameH {
		return false
	}
	_, pred := chooseIntraMode(lumaY, recY, bx, by, 32, 32, frameW, frameH)
	if pred == nil {
		return false
	}
	// Compute per-sample mean absolute deviation from the best
	// prediction. For a 32×32 block at MAD ≈ 20 the AC coefficients
	// routinely exceed the base+BR threshold of 15 at high quality
	// and the Golomb tail kicks in; splitting to 16×16 reduces each
	// sub-block's dynamic range and encodes more efficiently.
	sad := 0
	for r := 0; r < 32; r++ {
		srcRow := (by + r) * frameW
		predRow := r * 32
		for c := 0; c < 32; c++ {
			d := int(lumaY[srcRow+bx+c]) - int(pred[predRow+c])
			if d < 0 {
				d = -d
			}
			sad += d
		}
	}
	mad := sad / (32 * 32)
	return mad > 20
}

// chooseIntraMode runs a simple SAD-based search over the non-
// directional intra modes and returns the winner plus its prediction.
// It only considers modes whose neighbor requirements are satisfied.
func chooseIntraMode(lumaY, recY []uint8, bx, by, bw, bh, frameW, frameH int) (decoder.IntraMode, []uint8) {
	n := buildNeighbors(recY, bx, by, bw, bh, frameW, frameH)

	candidates := []decoder.IntraMode{decoder.DCPred}
	if n.HaveAbove {
		candidates = append(candidates, decoder.VPred)
	}
	if n.HaveLeft {
		candidates = append(candidates, decoder.HPred)
	}
	if n.HaveAbove && n.HaveLeft {
		candidates = append(candidates,
			decoder.PaethPred, decoder.SmoothPred,
			decoder.SmoothVPred, decoder.SmoothHPred,
			// Directional modes — now spec-correct since both sides
			// provide bw+bh extended neighbor arrays.
			decoder.D45Pred, decoder.D67Pred, decoder.D113Pred,
			decoder.D135Pred, decoder.D157Pred, decoder.D203Pred)
	}

	bestMode := decoder.DCPred
	var bestPred []uint8
	bestSAD := int(-1)
	for _, m := range candidates {
		pred := make([]uint8, bw*bh)
		if err := decoder.PredictIntra(pred, bw, bh, m, n); err != nil {
			continue
		}
		sad := 0
		for r := 0; r < bh; r++ {
			srcRow := (by + r) * frameW
			predRow := r * bw
			for c := 0; c < bw; c++ {
				d := int(lumaY[srcRow+bx+c]) - int(pred[predRow+c])
				if d < 0 {
					d = -d
				}
				sad += d
			}
		}
		if bestSAD < 0 || sad < bestSAD {
			bestSAD = sad
			bestMode = m
			bestPred = pred
		}
	}
	return bestMode, bestPred
}

// buildNeighbors assembles the above / left reference samples and
// availability flags needed by PredictIntra for the block at (bx, by).
// Matching the decoder's neighbor setup, both the standard (bw / bh)
// arrays and the extended (bw+bh each) arrays used by directional
// modes are populated from the reconstructed luma buffer with edge-
// extension when the frame boundary is reached.
func buildNeighbors(recY []uint8, bx, by, bw, bh, frameW, frameH int) *decoder.Neighbors {
	extLen := bw + bh
	n := &decoder.Neighbors{
		HaveAbove:     by > 0,
		HaveLeft:      bx > 0,
		BitDepth:      8,
		Above:         make([]uint8, bw),
		Left:          make([]uint8, bh),
		AboveExtended: make([]uint8, extLen),
		LeftExtended:  make([]uint8, extLen),
	}
	if n.HaveAbove {
		row := (by - 1) * frameW
		for c := 0; c < extLen; c++ {
			sx := bx + c
			if sx >= frameW {
				sx = frameW - 1
			}
			n.AboveExtended[c] = recY[row+sx]
		}
		copy(n.Above, n.AboveExtended[:bw])
	}
	if n.HaveLeft {
		for r := 0; r < extLen; r++ {
			sy := by + r
			if sy >= frameH {
				sy = frameH - 1
			}
			n.LeftExtended[r] = recY[sy*frameW+(bx-1)]
		}
		copy(n.Left, n.LeftExtended[:bh])
	}
	if n.HaveAbove && n.HaveLeft {
		n.AboveLeft = recY[(by-1)*frameW+(bx-1)]
	}
	return n
}

// writeChromaDCLeaf emits the chroma coefficient syntax for the two
// chroma planes at 4:2:0. For each plane it computes the 2D residual
// against the reconstructed-neighbor DC prediction, forward-transforms,
// quantizes, emits, and reconstructs.
func writeChromaDCLeaf(enc *entropy.Encoder,
	bx, by, bw, bh, cStrideW, cStrideH int,
	chromaU, chromaV []uint8,
	recU, recV []uint8,
	baseQ, qCtx, subX, subY int) {
	cx := bx >> subX
	cy := by >> subY
	cbw := bw >> subX
	cbh := bh >> subY
	if cbw < 1 {
		cbw = 1
	}
	if cbh < 1 {
		cbh = 1
	}
	txSizeIdx, nzMap, scan, txSize, txW, txH := selectEncTxParams(cbw, cbh)

	for plane := 0; plane < 2; plane++ {
		srcPlane := chromaU
		recPlane := recU
		pl := quant.PlaneU
		if plane == 1 {
			srcPlane = chromaV
			recPlane = recV
			pl = quant.PlaneV
		}

		// DC_PRED from reconstructed neighbors.
		pred := make([]uint8, cbw*cbh)
		dcPredBlock(pred, recPlane, cx, cy, cbw, cbh, cStrideW, cStrideH)

		var hasChromaResidual bool
		var chromaCoeffs []int32
		var chromaDequant []int32

		if srcPlane != nil {
			residual := make([]int32, txW*txH)
			for r := 0; r < cbh && cy+r < cStrideH; r++ {
				srcRow := (cy + r) * cStrideW
				predRow := r * cbw
				for c := 0; c < cbw && cx+c < cStrideW; c++ {
					residual[r*txW+c] = int32(srcPlane[srcRow+cx+c]) - int32(pred[predRow+c])
				}
			}

			if err := transform.Forward2D(residual, transform.DctDct, txSize); err == nil {
				enforceClampedScan(residual, txSize, txW, txH)

				qp := quant.Params{BaseQIndex: baseQ, BitDepth: 8}
				qv := qp.Compute(pl)
				chromaCoeffs = make([]int32, txW*txH)
				for i := range residual {
					chromaCoeffs[i] = quant.QuantizeCoeff(residual[i], i, qv)
				}

				for _, v := range chromaCoeffs {
					if v != 0 {
						hasChromaResidual = true
						break
					}
				}

				if hasChromaResidual {
					chromaDequant = make([]int32, txW*txH)
					for i, v := range chromaCoeffs {
						chromaDequant[i] = decoder.DequantCoeff(v, i, qv)
					}
				}
			}
		}

		if hasChromaResidual {
			WriteCoefficients(enc, chromaCoeffs, txSizeIdx, 1 /*chroma*/, qCtx, scan, nzMap, txW, txH)
			reconstructAndWrite(recPlane, pred, chromaDequant,
				cx, cy, cbw, cbh, txW, txH,
				transform.DctDct, txSize,
				cStrideW)
		} else {
			enc.EncodeSymbol(cdfs.DefaultTxbSkipCDF[clamp(txSizeIdx, 0, 4)][0], 1) // skip
			// Reconstruction for skip: prediction.
			writeBack(recPlane, pred, cx, cy, cbw, cbh, cStrideW)
		}
	}
}

// writeIntraTxTypeIfNeeded emits the ext_tx intra symbol for a luma
// block when its area is <= 32×32. Larger blocks skip the symbol and
// the decoder infers DCT_DCT per spec §6.10.15. We always emit
// DCT_DCT (raw symbol 0).
func writeIntraTxTypeIfNeeded(enc *entropy.Encoder, bw, bh, intraMode int) {
	area := bw * bh
	var cdfIdx int
	switch {
	case area <= 16*16:
		// txSet=1: 7-symbol CDF from DefaultIntraExtTxCDFSet1.
		cdfIdx = extTxSizeCtx(bw, bh)
		enc.EncodeSymbol(cdfs.DefaultIntraExtTxCDFSet1[cdfIdx][clamp(intraMode, 0, 12)], 0)
		return
	case area <= 32*32:
		// txSet=2: 5-symbol CDF from DefaultIntraExtTxCDFSet2.
		cdfIdx = extTxSizeCtx(bw, bh)
		enc.EncodeSymbol(cdfs.DefaultIntraExtTxCDFSet2[cdfIdx][clamp(intraMode, 0, 12)], 0)
		return
	}
}

// extTxSizeCtx mirrors decoder.ExtTxSizeCtx.
func extTxSizeCtx(bw, bh int) int {
	area := bw * bh
	switch {
	case area <= 4*4:
		return 0
	case area <= 8*8:
		return 1
	case area <= 16*16:
		return 2
	}
	return 3
}

// selectEncTxParams mirrors the decoder's selectTxParams for the
// block sizes the encoder currently emits. Returns the values needed
// by WriteCoefficients and Forward2D.
func selectEncTxParams(w, h int) (txSizeIdx int, nzMap []int8, scan []int, txSize transform.TxSize, txW, txH int) {
	switch {
	case w == 4 && h == 4:
		return 0, cdfs.NzMapCtxOffset4x4[:], transform.DefaultZigzagScan(4, 4), transform.Tx4x4, 4, 4
	case w == 8 && h == 8:
		return 1, cdfs.NzMapCtxOffset8x8[:], transform.DefaultZigzagScan(8, 8), transform.Tx8x8, 8, 8
	case w == 16 && h == 16:
		return 2, cdfs.NzMapCtxOffset16x16[:], transform.DefaultZigzagScan(16, 16), transform.Tx16x16, 16, 16
	case w == 32 && h == 32:
		return 3, cdfs.NzMapCtxOffset32x32[:], transform.DefaultZigzagScan(32, 32), transform.Tx32x32, 32, 32
	case w == 64 && h == 64:
		return 4, cdfs.NzMapCtxOffset32x32[:], transform.ClampedScan(32, 32, 64), transform.Tx64x64, 64, 64
	case w == 64 && h == 32:
		return 3, cdfs.NzMapCtxOffset32x32[:], transform.ClampedScan(32, 32, 64), transform.Tx64x32, 64, 32
	case w == 32 && h == 64:
		return 3, cdfs.NzMapCtxOffset32x32[:], transform.DefaultZigzagScan(32, 32), transform.Tx32x64, 32, 64
	}
	// Fallback to 32×32 for unknown shapes.
	return 3, cdfs.NzMapCtxOffset32x32[:], transform.DefaultZigzagScan(32, 32), transform.Tx32x32, 32, 32
}

// enforceClampedScan zeros out coefficients outside the top-left
// coded region for TX sizes that use a clamped scan per spec §7.7.3.
func enforceClampedScan(coeffs []int32, sz transform.TxSize, txW, txH int) {
	// TX_64×64 / TX_64×32 / TX_16×64 / TX_64×16 code only the top-left
	// 32×(min(h,32)) or (min(w,32))×32 subregion.
	var codedW, codedH int
	switch sz {
	case transform.Tx64x64:
		codedW, codedH = 32, 32
	case transform.Tx64x32:
		codedW, codedH = 32, 32
	case transform.Tx32x64:
		// Full default zigzag scan of 32×32, but the block is 32×64;
		// the decoder places coefficients only in the top half, which
		// is the whole block here — no extra clamping needed beyond
		// the scan limit.
		codedW, codedH = 32, 32
	case transform.Tx64x16:
		codedW, codedH = 32, 16
	case transform.Tx16x64:
		codedW, codedH = 16, 32
	default:
		return
	}
	if txW == codedW && txH == codedH {
		return
	}
	for r := 0; r < txH; r++ {
		for c := 0; c < txW; c++ {
			if r >= codedH || c >= codedW {
				coeffs[r*txW+c] = 0
			}
		}
	}
}

// dcPredBlock writes DC_PRED samples for an (bw × bh) block at (bx, by)
// using reconstructed neighbors from ref. Matches predict.DCPred
// semantics: averages available top-row + left-column samples, or
// 128 when both are missing.
func dcPredBlock(dst []uint8, ref []uint8, bx, by, bw, bh, frameW, frameH int) {
	haveAbove := by > 0
	haveLeft := bx > 0

	sum := 0
	n := 0
	if haveAbove {
		row := (by - 1) * frameW
		for c := 0; c < bw && bx+c < frameW; c++ {
			sum += int(ref[row+bx+c])
			n++
		}
	}
	if haveLeft {
		for r := 0; r < bh && by+r < frameH; r++ {
			sum += int(ref[(by+r)*frameW+(bx-1)])
			n++
		}
	}
	var dc uint8 = 128
	if n > 0 {
		dc = uint8((sum + n/2) / n)
	}
	for i := range dst {
		dst[i] = dc
	}
}

// reconstructAndWrite applies inverse 2D transform to dequant coefs,
// adds to pred, clips, and writes into ref at (bx, by) with stride.
func reconstructAndWrite(ref []uint8, pred []uint8, dequant []int32,
	bx, by, bw, bh, txW, txH int,
	txType transform.TxType, txSize transform.TxSize,
	stride int) {
	// Inverse2D mutates in place.
	resid := append([]int32(nil), dequant...)
	_ = transform.Inverse2D(resid, txType, txSize)
	for r := 0; r < bh; r++ {
		for c := 0; c < bw; c++ {
			v := int32(pred[r*bw+c]) + resid[r*txW+c]
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			ref[(by+r)*stride+(bx+c)] = uint8(v)
		}
	}
}

// writeBack copies src (bw*bh) into ref at (bx, by) with the given
// stride. Used when a block is skipped and reconstruction == pred.
func writeBack(ref []uint8, src []uint8, bx, by, bw, bh, stride int) {
	for r := 0; r < bh; r++ {
		copy(ref[(by+r)*stride+bx:(by+r)*stride+bx+bw], src[r*bw:r*bw+bw])
	}
}

// writeChromaSkipReconstruction fills the chroma reference buffers for
// a luma-skipped block. Pred for both U and V is 128 at frame corner,
// otherwise the reconstructed neighbors; we simply write 128 here.
func writeChromaSkipReconstruction(recU, recV []uint8,
	bx, by, bw, bh, cStrideW, cStrideH, subX, subY int) {
	cx := bx >> subX
	cy := by >> subY
	cbw := bw >> subX
	cbh := bh >> subY
	if cbw < 1 {
		cbw = 1
	}
	if cbh < 1 {
		cbh = 1
	}
	pred := make([]uint8, cbw*cbh)
	// Use reconstructed-neighbor DC pred for chroma skip.
	dcPredBlock(pred, recU, cx, cy, cbw, cbh, cStrideW, cStrideH)
	writeBack(recU, pred, cx, cy, cbw, cbh, cStrideW)
	dcPredBlock(pred, recV, cx, cy, cbw, cbh, cStrideW, cStrideH)
	writeBack(recV, pred, cx, cy, cbw, cbh, cStrideW)
}
