package decoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// BlockInput carries everything required to reconstruct a single transform
// block's worth of luma (or chroma) samples.
type BlockInput struct {
	W, H int       // block dimensions in samples
	Mode IntraMode // intra prediction mode
	TxType transform.TxType
	TxSize transform.TxSize
	// Coeffs are the dequantized AV1 transform coefficients in row-major
	// layout. They will NOT be touched if Skip is true.
	Coeffs []int32
	// Skip indicates the all-zero residual fast path; predict only.
	Skip bool
	Neighbors *Neighbors
}

// DecodeBlock reconstructs a single transform block end-to-end and writes
// the result into dst (length W*H). It runs the intra predictor, applies
// the inverse 2D transform to the supplied coefficients (unless skipped),
// and combines the two with clipping.
//
// Coeffs are expected to already be dequantized; raw quantized coefficients
// should be multiplied by the appropriate DC/AC dequantizers from
// [quant.Params.Compute] before calling this helper.
func DecodeBlock(dst []uint8, in *BlockInput) error {
	if in == nil {
		return fmt.Errorf("av1/decoder: nil BlockInput")
	}
	if len(dst) != in.W*in.H {
		return fmt.Errorf("av1/decoder: dst length %d, want %d", len(dst), in.W*in.H)
	}
	pred := make([]uint8, in.W*in.H)
	if err := PredictIntra(pred, in.W, in.H, in.Mode, in.Neighbors); err != nil {
		return err
	}
	if in.Skip {
		copy(dst, pred)
		return nil
	}
	if err := transform.Inverse2D(in.Coeffs, in.TxType, in.TxSize); err != nil {
		return err
	}
	ReconstructBlock(dst, pred, in.Coeffs, in.W, in.H)
	return nil
}

// DequantCoeff multiplies a single quantized coefficient by the DC or AC
// dequantizer per spec §7.12.3. pos == 0 selects DC; otherwise AC.
func DequantCoeff(quantized int32, pos int, vals quant.Values) int32 {
	if pos == 0 {
		return quantized * int32(vals.DC)
	}
	return quantized * int32(vals.AC)
}
