package decoder

// IntraMode enumerates AV1's per-block intra prediction modes (spec §6.4.1).
// The numeric values match the bitstream encoding so they can be passed
// directly to predict-package dispatch.
type IntraMode uint8

const (
	DCPred       IntraMode = 0
	VPred        IntraMode = 1
	HPred        IntraMode = 2
	D45Pred      IntraMode = 3
	D135Pred     IntraMode = 4
	D113Pred     IntraMode = 5
	D157Pred     IntraMode = 6
	D203Pred     IntraMode = 7
	D67Pred      IntraMode = 8
	SmoothPred   IntraMode = 9
	SmoothVPred  IntraMode = 10
	SmoothHPred  IntraMode = 11
	PaethPred    IntraMode = 12
	IntraModes   = 13
)

// IsDirectional reports whether the mode uses an angle-derivative scan
// pattern (D45..D67). These modes additionally take an angle_delta in
// {-3..3} from the bitstream.
func (m IntraMode) IsDirectional() bool {
	return m >= D45Pred && m <= D67Pred
}

// String returns the spec name.
func (m IntraMode) String() string {
	switch m {
	case DCPred:
		return "DC_PRED"
	case VPred:
		return "V_PRED"
	case HPred:
		return "H_PRED"
	case D45Pred:
		return "D45_PRED"
	case D135Pred:
		return "D135_PRED"
	case D113Pred:
		return "D113_PRED"
	case D157Pred:
		return "D157_PRED"
	case D203Pred:
		return "D203_PRED"
	case D67Pred:
		return "D67_PRED"
	case SmoothPred:
		return "SMOOTH_PRED"
	case SmoothVPred:
		return "SMOOTH_V_PRED"
	case SmoothHPred:
		return "SMOOTH_H_PRED"
	case PaethPred:
		return "PAETH_PRED"
	}
	return "UNKNOWN_PRED"
}

// UVMode includes the additional CFL_PRED on top of the 13 intra modes.
type UVMode uint8

const (
	UVMode_CFLPred UVMode = 13
	UVModes               = 14
)

// MIInfo is the mode-info grid resolution: AV1's smallest unit is 4x4
// luma samples, so block dimensions divide by 4 to give MI rows/cols.
//
// MIWidth and MIHeight return the number of MI cells the block occupies.
func (b BlockSize) MIWidth() int  { return blockWidths[b] >> 2 }
func (b BlockSize) MIHeight() int { return blockHeights[b] >> 2 }

// SubsampledMIDims returns the MI dimensions for the chroma plane given
// the subsampling factor (0 for full, 1 for half on that axis).
func (b BlockSize) SubsampledMIDims(subX, subY int) (mw, mh int) {
	mw = b.MIWidth() >> subX
	if mw < 1 {
		mw = 1
	}
	mh = b.MIHeight() >> subY
	if mh < 1 {
		mh = 1
	}
	return
}

// MaxTXSize returns the largest transform size that fits in the given
// block, capped at 64×64 (AV1 hard ceiling). Used by the bitstream's
// implicit-TX path when the encoder did not signal an explicit TX size.
func MaxTXSize(b BlockSize) (txW, txH int) {
	w := b.Width()
	h := b.Height()
	if w > 64 {
		w = 64
	}
	if h > 64 {
		h = 64
	}
	return w, h
}
