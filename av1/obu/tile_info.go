package obu

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/bitio"
)

// TileInfo decodes tile_info() (spec §5.9.15).
type TileInfo struct {
	UniformTileSpacing bool
	TileColsLog2       uint8
	TileRowsLog2       uint8
	TileCols           uint16
	TileRows           uint16
	MiColStarts        []uint32
	MiRowStarts        []uint32
	ContextUpdateTileID uint16
	TileSizeBytesMinus1 uint8
}

// parseTileInfo fills ti from r. It requires fh.FrameWidth/Height to be set
// (done earlier in the uncompressed header).
func parseTileInfo(r *bitio.Reader, ti *TileInfo, sh *SequenceHeader, fh *FrameHeader) error {
	sbSize := uint32(64)
	if sh.Use128x128Superblock {
		sbSize = 128
	}
	// Superblocks per dimension, rounded up.
	miCols := (fh.FrameWidth + 7) >> 3
	miRows := (fh.FrameHeight + 7) >> 3
	sbRowsLog2 := tileLog2(sbSize>>3, miRows)
	sbColsLog2 := tileLog2(sbSize>>3, miCols)
	sbCols := (miCols + (sbSize>>3) - 1) / (sbSize >> 3)
	sbRows := (miRows + (sbSize>>3) - 1) / (sbSize >> 3)
	// Max tile widths/heights per spec.
	maxTileWidthSb := uint32(MaxTileWidth) / sbSize
	maxTileAreaSb := uint32(4096*2304) / (sbSize * sbSize)
	_ = maxTileAreaSb // used as derived upper bound; unused here but kept for clarity
	minLog2TileCols := tileLog2(maxTileWidthSb, sbCols)
	maxLog2TileCols := tileLog2(1, minOf(sbCols, MaxTileCols))
	maxLog2TileRows := tileLog2(1, minOf(sbRows, MaxTileRows))
	minLog2Tiles := maxU32(minLog2TileCols,
		tileLog2(maxTileAreaSb, sbRows*sbCols))
	_ = sbRowsLog2
	_ = sbColsLog2

	ti.UniformTileSpacing = r.F(1) == 1
	if ti.UniformTileSpacing {
		ti.TileColsLog2 = uint8(minLog2TileCols)
		for ti.TileColsLog2 < uint8(maxLog2TileCols) {
			if r.F(1) == 1 {
				ti.TileColsLog2++
			} else {
				break
			}
		}
		tileWidthSb := (sbCols + (1 << ti.TileColsLog2) - 1) >> ti.TileColsLog2
		ti.MiColStarts = tileStarts(sbCols, tileWidthSb, sbSize>>3)
		ti.TileCols = uint16(len(ti.MiColStarts) - 1)

		minLog2TileRows := maxU32(minLog2Tiles-uint32(ti.TileColsLog2), 0)
		ti.TileRowsLog2 = uint8(minLog2TileRows)
		for ti.TileRowsLog2 < uint8(maxLog2TileRows) {
			if r.F(1) == 1 {
				ti.TileRowsLog2++
			} else {
				break
			}
		}
		tileHeightSb := (sbRows + (1 << ti.TileRowsLog2) - 1) >> ti.TileRowsLog2
		ti.MiRowStarts = tileStarts(sbRows, tileHeightSb, sbSize>>3)
		ti.TileRows = uint16(len(ti.MiRowStarts) - 1)
	} else {
		startSb := uint32(0)
		ti.MiColStarts = []uint32{0}
		for startSb < sbCols {
			remain := sbCols - startSb
			maxW := minU32(remain, maxTileWidthSb)
			widthSb := r.Ns(maxW) + 1
			startSb += widthSb
			ti.MiColStarts = append(ti.MiColStarts, startSb*(sbSize>>3))
		}
		// Clamp final entry to picture size.
		ti.MiColStarts[len(ti.MiColStarts)-1] = miCols
		ti.TileCols = uint16(len(ti.MiColStarts) - 1)
		ti.TileColsLog2 = uint8(ceilLog2(uint32(ti.TileCols)))

		startSb = 0
		ti.MiRowStarts = []uint32{0}
		// Per-spec max tile height depends on total tile area; approximate
		// with the MaxTileHeight bound.
		maxTileHeightSb := maxU32(4096*2304/(sbSize*sbSize)/maxU32(uint32(ti.TileCols), 1), 1)
		for startSb < sbRows {
			remain := sbRows - startSb
			maxH := minU32(remain, maxTileHeightSb)
			heightSb := r.Ns(maxH) + 1
			startSb += heightSb
			ti.MiRowStarts = append(ti.MiRowStarts, startSb*(sbSize>>3))
		}
		ti.MiRowStarts[len(ti.MiRowStarts)-1] = miRows
		ti.TileRows = uint16(len(ti.MiRowStarts) - 1)
		ti.TileRowsLog2 = uint8(ceilLog2(uint32(ti.TileRows)))
	}

	if ti.TileColsLog2+ti.TileRowsLog2 > 0 {
		ti.ContextUpdateTileID = uint16(r.F(uint(ti.TileColsLog2) + uint(ti.TileRowsLog2)))
		ti.TileSizeBytesMinus1 = uint8(r.F(2))
	}
	if err := r.Err(); err != nil {
		return fmt.Errorf("%w: tile_info: %w", ErrMalformed, err)
	}
	return nil
}

// tileLog2 returns the smallest k such that blkSize << k >= target.
func tileLog2(blkSize, target uint32) uint32 {
	k := uint32(0)
	for blkSize<<k < target {
		k++
	}
	return k
}

// tileStarts generates tile start positions in mi units given the number of
// superblocks per dimension, the tile size in superblocks, and mi per sb.
func tileStarts(nSb, sizeSb, miPerSb uint32) []uint32 {
	out := []uint32{}
	for start := uint32(0); start < nSb; start += sizeSb {
		out = append(out, start*miPerSb)
	}
	out = append(out, nSb*miPerSb)
	return out
}

func ceilLog2(x uint32) uint32 {
	if x <= 1 {
		return 0
	}
	k := uint32(0)
	x--
	for x > 0 {
		x >>= 1
		k++
	}
	return k
}

func minU32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func maxU32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func minOf(a, b uint32) uint32 { return minU32(a, b) }
