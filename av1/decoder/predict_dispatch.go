package decoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/predict"
)

// Neighbors carries the reconstructed samples adjacent to a block, plus
// availability flags. above / left have at least w / h samples; the
// directional predictors require longer extensions and the caller is
// expected to satisfy that explicitly.
type Neighbors struct {
	Above       []uint8
	Left        []uint8
	AboveLeft   uint8
	HaveAbove   bool
	HaveLeft    bool
	BitDepth    int

	// AboveExtended and LeftExtended are the longer forms used by the
	// directional predictors (D45 needs above[0..w+h-1], etc.). When
	// non-nil they replace Above / Left for those modes.
	AboveExtended []uint8
	LeftExtended  []uint8
}

// PredictIntra runs the requested intra-prediction mode for a block of
// size w×h, writing the result into dst (length w*h).
//
// Modes that depend on extended neighbor samples will use
// Neighbors.AboveExtended / LeftExtended when provided. Returns an error
// for modes that are not yet implemented (filter intra, recursive intra,
// CFL — see [PredictCFL] for the latter).
func PredictIntra(dst []uint8, w, h int, mode IntraMode, n *Neighbors) error {
	switch mode {
	case DCPred:
		predict.DCPred(dst, w, h, n.Above, n.Left, n.HaveAbove, n.HaveLeft, n.BitDepth)
	case VPred:
		if !n.HaveAbove {
			return fmt.Errorf("V_PRED requires above samples")
		}
		predict.VPred(dst, w, h, n.Above)
	case HPred:
		if !n.HaveLeft {
			return fmt.Errorf("H_PRED requires left samples")
		}
		predict.HPred(dst, w, h, n.Left)
	case PaethPred:
		if !n.HaveAbove || !n.HaveLeft {
			return fmt.Errorf("PAETH_PRED requires both above and left")
		}
		predict.PaethPred(dst, w, h, n.Above, n.Left, n.AboveLeft)
	case SmoothPred:
		predict.SmoothPred(dst, w, h, n.Above, n.Left)
	case SmoothVPred:
		predict.SmoothVPred(dst, w, h, n.Above, n.Left)
	case SmoothHPred:
		predict.SmoothHPred(dst, w, h, n.Above, n.Left)
	case D45Pred, D67Pred, D113Pred, D135Pred, D157Pred, D203Pred:
		above := n.AboveExtended
		if above == nil {
			above = n.Above
		}
		left := n.LeftExtended
		if left == nil {
			left = n.Left
		}
		angle := predict.ModeToAngleMap[mode]
		predict.DirectionalPred(dst, w, h, above, left, angle)
	}
	return nil
}
