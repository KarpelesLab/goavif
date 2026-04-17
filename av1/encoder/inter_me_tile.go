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
// the reference planes (refW × refH). Only 4:2:0 single-reference
// integer-pel ME is supported today. searchRange is the per-axis
// half-window in pixels (e.g. 16 → ±16 pel search).
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
	miCols := (width + 3) >> 2
	miRows := (height + 3) >> 2
	inter := make([]uint8, miCols*miRows)
	refYStride := refW
	refCStride := refW >> 1
	srcYStride := width

	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			writePartitionSymbol(&enc, 3, 0, 3 /* SPLIT */)
			for _, off := range [4][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}} {
				bx := x + off[0]
				by := y + off[1]
				writePartitionSymbol(&enc, 2, 0, 0 /* NONE */)
				mv := DiamondSearchMV(srcY, srcYStride, bx, by, 32, 32,
					refY, refW, refH, refYStride, searchRange)
				mv = SubPelRefineMV(srcY, srcYStride, bx, by, 32, 32,
					refY, refW, refH, refYStride, mv)
				writeInterResidualBlock(&enc, bx, by, 32, 32, mv,
					srcY, srcU, srcV,
					refY, refU, refV, refW, refH, refYStride, refCStride,
					inter, miCols, miRows, baseQ)
			}
		}
	}
	return enc.Finish(), nil
}
