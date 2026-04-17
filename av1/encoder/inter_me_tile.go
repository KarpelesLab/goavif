package encoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// WriteInterMETile emits an inter tile where every 32×32 block runs
// motion estimation independently against the reference frame, then
// encodes the best MV + residual. This is the primary entry point
// for encoding a real AVIS inter frame.
//
// srcY / srcU / srcV are the source planes; refY / refU / refV are
// the reference planes (refW × refH). Chroma subsampling is taken
// from sh.Color (supports 4:2:0 / 4:2:2 / 4:4:4). searchRange is the
// per-axis half-window in pixels.
//
// When a 32×32 block's best-MV SAD is above a split threshold, the
// block is further split into four 16×16 sub-blocks, each with its
// own MV. This improves motion tracking in complex regions at a
// small bitstream cost.
func WriteInterMETile(width, height int,
	fh *obu.FrameHeader, sh *obu.SequenceHeader,
	srcY, srcU, srcV []uint8,
	refY, refU, refV []uint8,
	refW, refH int,
	searchRange int,
) ([]byte, error) {
	if sh == nil || fh == nil {
		return nil, fmt.Errorf("encoder: nil sh / fh")
	}
	if width%64 != 0 || height%64 != 0 {
		return nil, fmt.Errorf("encoder: WriteInterMETile requires 64-aligned dims, got %dx%d", width, height)
	}
	var enc entropy.Encoder
	enc.Init(!fh.DisableCDFUpdate)

	sbSize := 64
	baseQ := int(fh.Quant.BaseQIndex)
	subX := int(sh.Color.SubsamplingX)
	subY := int(sh.Color.SubsamplingY)
	miCols := (width + 3) >> 2
	miRows := (height + 3) >> 2
	inter := make([]uint8, miCols*miRows)
	refYStride := refW
	refCStride := refW >> uint(subX)
	srcYStride := width

	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			writePartitionSymbol(&enc, 3, 0, 3 /* SPLIT */)
			for _, off := range [4][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}} {
				bx := x + off[0]
				by := y + off[1]
				encodeInter32(&enc, bx, by,
					srcY, srcU, srcV, srcYStride,
					refY, refU, refV, refW, refH, refYStride, refCStride,
					inter, miCols, miRows, baseQ, searchRange, subX, subY)
			}
		}
	}
	return enc.Finish(), nil
}

// encodeInter32 handles a single 32×32 block: runs ME, optionally
// splits to 16×16 if the residual looks high, and emits the resulting
// partition symbol + per-sub-block inter payload.
func encodeInter32(enc *entropy.Encoder, bx, by int,
	srcY, srcU, srcV []uint8, srcYStride int,
	refY, refU, refV []uint8,
	refW, refH, refYStride, refCStride int,
	inter []uint8, miCols, miRows, baseQ, searchRange int,
	subX, subY int,
) {
	mv := DiamondSearchMV(srcY, srcYStride, bx, by, 32, 32,
		refY, refW, refH, refYStride, searchRange)
	mv = SubPelRefineMV(srcY, srcYStride, bx, by, 32, 32,
		refY, refW, refH, refYStride, mv)

	// Evaluate the 32×32 SAD after MC. If it's small, stay at 32×32.
	// Threshold chosen so that 32×32 SAD/pixel ≈ 12 (a typical
	// acceptable match for smooth content). Above that we split.
	sad32 := sadForMV(srcY, srcYStride, bx, by, 32, 32,
		refY, refW, refH, refYStride, mv)
	splitThreshold := 32 * 32 * 12 // ~12 per pixel
	if sad32 <= splitThreshold {
		writePartitionSymbol(enc, 2, 0, 0 /* NONE */)
		writeInterResidualBlock(enc, bx, by, 32, 32, mv,
			srcY, srcU, srcV,
			refY, refU, refV, refW, refH, refYStride, refCStride,
			inter, miCols, miRows, baseQ, subX, subY)
		return
	}
	// Split path — emit PARTITION_SPLIT at bsl=2 and re-run ME per
	// 16×16 sub-block.
	writePartitionSymbol(enc, 2, 0, 3 /* SPLIT */)
	for _, sub := range [4][2]int{{0, 0}, {16, 0}, {0, 16}, {16, 16}} {
		sx := bx + sub[0]
		sy := by + sub[1]
		mv16 := DiamondSearchMV(srcY, srcYStride, sx, sy, 16, 16,
			refY, refW, refH, refYStride, searchRange)
		mv16 = SubPelRefineMV(srcY, srcYStride, sx, sy, 16, 16,
			refY, refW, refH, refYStride, mv16)
		writePartitionSymbol(enc, 1, 0, 0 /* NONE */)
		writeInterResidualBlock(enc, sx, sy, 16, 16, mv16,
			srcY, srcU, srcV,
			refY, refU, refV, refW, refH, refYStride, refCStride,
			inter, miCols, miRows, baseQ, subX, subY)
	}
}
