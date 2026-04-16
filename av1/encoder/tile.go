// Package encoder assembles AV1 bitstreams for the goavif encoder. It
// pairs with [av1/decoder] at the syntax level: whatever encoder
// produces should be directly consumable by decoder.Decode.
//
// The current encoder is a minimal-feature first-pass: it emits all
// blocks as PARTITION_NONE + DC_PRED + skip, which avoids the
// coefficient-coding path entirely. The result decodes to a
// constant-chroma mid-grey image — visually useless but structurally
// valid, providing a substrate for future encoder work.
package encoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// WriteIntraOnlyTile emits a minimal tile payload for an intra-only
// keyframe of dimension (width, height). Every superblock is encoded
// as a single 64×64 (or 128×128) PARTITION_NONE block with DC_PRED
// y-mode, DC_PRED uv-mode, skip=true — no residual.
//
// The tile_size header (leb128 prefix) is NOT included; callers
// should prepend it per AV1 §5.11.1 when writing the tile group OBU.
func WriteIntraOnlyTile(width, height int, fh *obu.FrameHeader, sh *obu.SequenceHeader) ([]byte, error) {
	if sh == nil || fh == nil {
		return nil, fmt.Errorf("encoder: nil sh / fh")
	}

	var enc entropy.Encoder
	enc.Init(!fh.DisableCDFUpdate)

	sbSize := 64
	if sh.Use128x128Superblock {
		sbSize = 128
	}
	// Walk superblocks in raster order.
	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			if err := writeSuperblock(&enc, x, y, sbSize, sh); err != nil {
				return nil, err
			}
		}
	}
	return enc.Finish(), nil
}

// writeSuperblock emits the syntax for a single SB using PARTITION_NONE
// at the top with DC_PRED + skip for every leaf block.
func writeSuperblock(enc *entropy.Encoder, x, y, sbSize int, sh *obu.SequenceHeader) error {
	// PARTITION_NONE = 0. Partition CDF index depends on block-size log
	// and above/left context — at the top of the SB with no neighbors
	// signaled, use context 0.
	writePartitionNone(enc, sbSize)
	// Leaf block decoded now.
	writeDCSkipLeaf(enc, sh)
	// cdef_idx not signaled (fh.Cdef.CdefBits == 0 in our sequence
	// header) — skip.
	return nil
}

func writePartitionNone(enc *entropy.Encoder, bs int) {
	// bslCtx = blockSizeLog bucket. For 64x64 it's 2, for 128x128 it's 3.
	// See decoder.decodePartitionNode / blockSizeLog. bslCtx*4 + ctx.
	bslCtx := 2 // 64x64
	if bs == 128 {
		bslCtx = 3
	}
	ctx := 0 // above/left = 0 at SB start
	cdfIdx := bslCtx*4 + ctx
	if cdfIdx >= len(cdfs.DefaultPartitionCDF) {
		return
	}
	// Use a local CDF copy so update behavior doesn't leak into the
	// (shared) default.
	cdf := append(cdfs.CDF(nil), cdfs.DefaultPartitionCDF[cdfIdx]...)
	// PARTITION_NONE = symbol 0.
	enc.EncodeSymbol(cdf, 0)
}

// writeDCSkipLeaf emits the mode + skip symbols for a single leaf
// block, assuming DC_PRED for both Y and UV, no segmentation, skip=1.
func writeDCSkipLeaf(enc *entropy.Encoder, sh *obu.SequenceHeader) {
	// Y intra mode = DC_PRED = 0. kf_y_mode CDF indexed by above/left
	// context buckets (5 each). Start-of-frame: both = 0 (DC bucket).
	kfCDF := append(cdfs.CDF(nil), cdfs.DefaultKfYModeCDF[0][0]...)
	enc.EncodeSymbol(kfCDF, 0)

	// skip flag = 1, ctx = 0.
	skipCDF := append(cdfs.CDF(nil), cdfs.DefaultSkipCDF[0]...)
	enc.EncodeSymbol(skipCDF, 1)

	// UV mode = DC_PRED = 0. cfl_allowed = true.
	if !sh.Color.Monochrome {
		uvCDF := append(cdfs.CDF(nil), cdfs.DefaultUVModeCDF[1][0]...)
		enc.EncodeSymbol(uvCDF, 0)
	}
}
