package decoder

import (
	"errors"
	"fmt"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// ErrCoeffDecodeUnimplemented is returned when the tile decoder successfully
// reads partition + mode symbols but cannot yet decode coefficient levels.
var ErrCoeffDecodeUnimplemented = errors.New("av1/decoder: coefficient decoding not yet implemented")

// DecodedBlock holds the mode-level information decoded from the bitstream
// for a single coding block. Coefficient data is not yet populated.
type DecodedBlock struct {
	X, Y   int
	W, H   int
	Mode   IntraMode
	UVMode IntraMode // or CFL (13)
	Skip   bool
}

// TileDecoder reads AV1 symbol-coded syntax from a single tile's byte
// span using the entropy decoder and the default CDF tables.
type TileDecoder struct {
	dec   entropy.Decoder
	fh    *obu.FrameHeader
	sh    *obu.SequenceHeader
	sbSize int

	// CDFs — mutable copies for per-tile adaptation.
	partitionCDF [20]cdfs.CDF
	kfYModeCDF   [5][5]cdfs.CDF
	uvModeCDF    [2][13]cdfs.CDF
	angleDeltaCDF [8]cdfs.CDF
	skipCDF      [3]cdfs.CDF
}

// NewTileDecoder initializes a tile decoder for the given tile data.
func NewTileDecoder(tileData []byte, fh *obu.FrameHeader, sh *obu.SequenceHeader) (*TileDecoder, error) {
	td := &TileDecoder{
		fh: fh,
		sh: sh,
	}
	if sh.Use128x128Superblock {
		td.sbSize = 128
	} else {
		td.sbSize = 64
	}
	allowUpdate := !fh.DisableCDFUpdate
	if err := td.dec.Init(tileData, len(tileData), allowUpdate); err != nil {
		return nil, fmt.Errorf("tile decoder init: %w", err)
	}
	td.initCDFs()
	return td, nil
}

// initCDFs copies the default CDFs into mutable per-tile state. The entropy
// decoder updates them in-place when allow_update_cdf is true.
func (td *TileDecoder) initCDFs() {
	for i := range cdfs.DefaultPartitionCDF {
		td.partitionCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultPartitionCDF[i]...)
	}
	for a := range cdfs.DefaultKfYModeCDF {
		for l := range cdfs.DefaultKfYModeCDF[a] {
			td.kfYModeCDF[a][l] = append(cdfs.CDF(nil), cdfs.DefaultKfYModeCDF[a][l]...)
		}
	}
	for c := range cdfs.DefaultUVModeCDF {
		for m := range cdfs.DefaultUVModeCDF[c] {
			td.uvModeCDF[c][m] = append(cdfs.CDF(nil), cdfs.DefaultUVModeCDF[c][m]...)
		}
	}
	for i := range cdfs.DefaultAngleDeltaCDF {
		td.angleDeltaCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultAngleDeltaCDF[i]...)
	}
	for i := range cdfs.DefaultSkipCDF {
		td.skipCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultSkipCDF[i]...)
	}
}

// DecodePartition reads a partition symbol for a square block of the given
// size class and left/above context.
func (td *TileDecoder) DecodePartition(bslCtx int, ctx int) int {
	cdfIdx := bslCtx*4 + ctx
	if cdfIdx >= len(td.partitionCDF) {
		return 0
	}
	return td.dec.DecodeSymbol(td.partitionCDF[cdfIdx])
}

// DecodeIntraYMode reads the Y-plane intra mode for a KEY_FRAME block.
// aboveCtx and leftCtx are the 5-bucket mode contexts of the neighbors.
func (td *TileDecoder) DecodeIntraYMode(aboveCtx, leftCtx int) IntraMode {
	if aboveCtx >= 5 {
		aboveCtx = 4
	}
	if leftCtx >= 5 {
		leftCtx = 4
	}
	return IntraMode(td.dec.DecodeSymbol(td.kfYModeCDF[aboveCtx][leftCtx]))
}

// DecodeUVMode reads the UV-plane intra mode given the Y mode and whether
// CFL is allowed.
func (td *TileDecoder) DecodeUVMode(yMode IntraMode, cflAllowed bool) IntraMode {
	cflIdx := 0
	if cflAllowed {
		cflIdx = 1
	}
	return IntraMode(td.dec.DecodeSymbol(td.uvModeCDF[cflIdx][yMode]))
}

// DecodeAngleDelta reads the angle delta for a directional mode.
// dirIdx is the directional mode index (yMode - D45Pred, in 0..7).
func (td *TileDecoder) DecodeAngleDelta(dirIdx int) int {
	if dirIdx < 0 || dirIdx >= 8 {
		return 0
	}
	return td.dec.DecodeSymbol(td.angleDeltaCDF[dirIdx]) - 3
}

// DecodeSkip reads the skip flag given a context index (0..2).
func (td *TileDecoder) DecodeSkip(ctx int) bool {
	if ctx >= 3 {
		ctx = 2
	}
	return td.dec.DecodeSymbol(td.skipCDF[ctx]) != 0
}

// Err returns any latched error from the entropy decoder.
func (td *TileDecoder) Err() error { return td.dec.Err() }
