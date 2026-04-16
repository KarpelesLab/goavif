package decoder

import (
	"github.com/KarpelesLab/goavif/av1/predict"
)

// fillHalf writes a half-range sample (2^(bd-1)) into every cell of dst.
// bd defaults to 8 when <= 0 so callers with un-initialized Neighbors
// don't hit a negative-shift panic.
func fillHalf(dst []uint8, w, h, bd int) {
	if bd <= 0 {
		bd = 8
	}
	half := uint8(1 << uint(bd-1))
	for i := range dst[:w*h] {
		dst[i] = half
	}
}

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

// Neighbors16 is the uint16 counterpart of [Neighbors] used by the
// 10/12-bit intra path.
type Neighbors16 struct {
	Above       []uint16
	Left        []uint16
	AboveLeft   uint16
	HaveAbove   bool
	HaveLeft    bool
	BitDepth    int

	AboveExtended []uint16
	LeftExtended  []uint16
}

// fillHalf16 writes a half-range uint16 sample into dst.
func fillHalf16(dst []uint16, w, h, bd int) {
	if bd <= 0 {
		bd = 8
	}
	half := uint16(1) << uint(bd-1)
	for i := range dst[:w*h] {
		dst[i] = half
	}
}

// PredictIntra16 is the uint16 counterpart of [PredictIntra]. It
// dispatches to the predict package's uint16 primitives for the
// 10/12-bit decode pipeline.
func PredictIntra16(dst []uint16, w, h int, mode IntraMode, n *Neighbors16) error {
	switch mode {
	case DCPred:
		predict.DCPred16(dst, w, h, n.Above, n.Left, n.HaveAbove, n.HaveLeft, n.BitDepth)
	case VPred:
		if !n.HaveAbove {
			fillHalf16(dst, w, h, n.BitDepth)
			return nil
		}
		predict.VPred16(dst, w, h, n.Above)
	case HPred:
		if !n.HaveLeft {
			fillHalf16(dst, w, h, n.BitDepth)
			return nil
		}
		predict.HPred16(dst, w, h, n.Left)
	case PaethPred:
		if !n.HaveAbove || !n.HaveLeft {
			fillHalf16(dst, w, h, n.BitDepth)
			return nil
		}
		predict.PaethPred16(dst, w, h, n.Above, n.Left, n.AboveLeft)
	case SmoothPred:
		predict.SmoothPred16(dst, w, h, n.Above, n.Left)
	case SmoothVPred:
		predict.SmoothVPred16(dst, w, h, n.Above, n.Left)
	case SmoothHPred:
		predict.SmoothHPred16(dst, w, h, n.Above, n.Left)
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
		predict.DirectionalPred16(dst, w, h, above, left, angle, n.BitDepth)
	}
	return nil
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
		// When above is unavailable (top frame edge) the spec substitutes
		// a half-range value so the prediction is well-defined.
		if !n.HaveAbove {
			fillHalf(dst, w, h, n.BitDepth)
			return nil
		}
		predict.VPred(dst, w, h, n.Above)
	case HPred:
		if !n.HaveLeft {
			fillHalf(dst, w, h, n.BitDepth)
			return nil
		}
		predict.HPred(dst, w, h, n.Left)
	case PaethPred:
		if !n.HaveAbove || !n.HaveLeft {
			fillHalf(dst, w, h, n.BitDepth)
			return nil
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
