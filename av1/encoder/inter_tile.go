package encoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// WriteInterCopyTile emits the tile-group payload for an inter frame
// that is a bit-for-bit copy of the reference frame: every block is
// is_inter=1, single_ref=LAST, inter_mode=NEWMV with MV=(0,0),
// skip_txfm=1. The decoder runs motion compensation against the
// reference at zero offset and skips the residual, so the output
// matches the reference exactly.
//
// Intended for roundtrip testing of the inter decode path — it
// produces minimal but spec-structured inter bitstreams.
func WriteInterCopyTile(width, height int, fh *obu.FrameHeader, sh *obu.SequenceHeader) ([]byte, error) {
	if sh == nil || fh == nil {
		return nil, fmt.Errorf("encoder: nil sh / fh")
	}
	var enc entropy.Encoder
	enc.Init(!fh.DisableCDFUpdate)

	sbSize := 64
	if sh.Use128x128Superblock {
		sbSize = 128
	}

	// Track per-MI is_inter state for is_inter CDF context. 4-pixel
	// MI grid: inter[miRow*miCols + miCol] = 1 once the block there
	// has been written as inter.
	miCols := (width + 3) >> 2
	miRows := (height + 3) >> 2
	inter := make([]uint8, miCols*miRows)

	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			if x+sbSize <= width && y+sbSize <= height {
				// Full-SB split into four 32×32 leaves — same
				// structure as the intra encoder.
				writePartitionSymbol(&enc, 3, 0, 3 /* SPLIT */)
				for _, off := range [4][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}} {
					bx := x + off[0]
					by := y + off[1]
					writePartitionSymbol(&enc, 2, 0, 0 /* NONE */)
					writeInterSkipBlock(&enc, sh, bx, by, 32, 32, inter, miCols, miRows)
				}
				continue
			}
			// Fallback PARTITION_NONE at the SB size (last row/column).
			bw := sbSize
			bh := sbSize
			if x+bw > width {
				bw = width - x
			}
			if y+bh > height {
				bh = height - y
			}
			writePartitionSymbol(&enc, partitionBsl(sbSize), 0, 0)
			writeInterSkipBlock(&enc, sh, x, y, bw, bh, inter, miCols, miRows)
		}
	}
	return enc.Finish(), nil
}

// writeInterSkipBlock emits the symbols for a single inter block
// that is a zero-MV / skip copy from the reference. Matches the
// decoder's decodeInterLeafBlock read order for is_inter=true +
// single-ref LAST + NEWMV + zero MV + skip_txfm.
func writeInterSkipBlock(enc *entropy.Encoder, sh *obu.SequenceHeader,
	bx, by, bw, bh int, inter []uint8, miCols, miRows int) {
	// is_inter context matches decoder.ReadIsInter: above / left
	// neighbor's inter flag determines the CDF.
	miCol := bx >> 2
	miRow := by >> 2
	aboveIsInter := miRow > 0 && inter[(miRow-1)*miCols+miCol] != 0
	leftIsInter := miCol > 0 && inter[miRow*miCols+(miCol-1)] != 0
	ctx := 0
	if aboveIsInter && leftIsInter {
		ctx = 3
	} else if aboveIsInter || leftIsInter {
		ctx = 1
	}
	enc.EncodeSymbol(cdfs.DefaultIsInterCDF[ctx], 1)

	// single_ref tree — LAST path. Decoder reads ctx=1.
	enc.EncodeSymbol(cdfs.DefaultSingleRefCDF[1][0], 0)
	enc.EncodeSymbol(cdfs.DefaultSingleRefCDF[1][1], 0)

	// Inter mode: NEWMV — first bit of the newmv tree is 0.
	enc.EncodeSymbol(cdfs.DefaultNewMvCDF[0], 0)

	// MV = (0, 0) via mv_joint = MV_JOINT_ZERO.
	enc.EncodeSymbol(cdfs.DefaultMvJointCDF, int(decoder.MVJointZero))

	// skip_txfm = 1.
	enc.EncodeSymbol(cdfs.DefaultSkipCDF[0], 1)

	// Mark every MI cell the block occupies as inter for the
	// neighbors of following blocks.
	miW := (bw + 3) >> 2
	miH := (bh + 3) >> 2
	for r := 0; r < miH && miRow+r < miRows; r++ {
		for c := 0; c < miW && miCol+c < miCols; c++ {
			inter[(miRow+r)*miCols+(miCol+c)] = 1
		}
	}

	_ = sh
}
