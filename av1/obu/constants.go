package obu

// FrameType values per spec §6.8.2.
type FrameType uint8

const (
	KeyFrame       FrameType = 0
	InterFrame     FrameType = 1
	IntraOnlyFrame FrameType = 2
	SwitchFrame    FrameType = 3
)

// String returns the spec name of the frame type.
func (f FrameType) String() string {
	switch f {
	case KeyFrame:
		return "KEY_FRAME"
	case InterFrame:
		return "INTER_FRAME"
	case IntraOnlyFrame:
		return "INTRA_ONLY_FRAME"
	case SwitchFrame:
		return "SWITCH_FRAME"
	}
	return "UNKNOWN_FRAME"
}

// IsIntra reports whether the frame type is KEY_FRAME or INTRA_ONLY_FRAME,
// in which case the spec's FrameIsIntra derived variable is true.
func (f FrameType) IsIntra() bool {
	return f == KeyFrame || f == IntraOnlyFrame
}

// Spec constants from §3.
const (
	NumRefFrames          = 8
	RefsPerFrame          = 7
	TotalRefsPerFrame     = 8
	BlockSizeGroups       = 4
	MaxTileWidth          = 4096
	MaxTileAreaLog2       = 23 // floor(log2(MaxTileArea)) not used directly
	MaxTileCols           = 64
	MaxTileRows           = 64
	MaxNumOperatingPoints = 32
	PrimaryRefNone        = 7
	MaxSegments           = 8
	SegLvlMax             = 8
	SegLvlAltQ            = 0
	SegLvlAltLF_Y_V       = 1
	SegLvlAltLF_Y_H       = 2
	SegLvlAltLF_U         = 3
	SegLvlAltLF_V         = 4
	SegLvlRefFrame        = 5
	SegLvlSkip            = 6
	SegLvlGlobalMv        = 7
	MaxLoopFilter         = 63
)

// Segmentation feature bit widths — Table from spec §6.8.13.
// Signed[f] and Bits[f] determine how to encode segmentation_feature_data.
var (
	segFeatureSigned = [SegLvlMax]bool{
		true,  // SEG_LVL_ALT_Q
		true,  // SEG_LVL_ALT_LF_Y_V
		true,  // SEG_LVL_ALT_LF_Y_H
		true,  // SEG_LVL_ALT_LF_U
		true,  // SEG_LVL_ALT_LF_V
		false, // SEG_LVL_REF_FRAME
		false, // SEG_LVL_SKIP
		false, // SEG_LVL_GLOBAL_MV
	}
	segFeatureBits = [SegLvlMax]uint{
		8, // SEG_LVL_ALT_Q
		6, // SEG_LVL_ALT_LF_Y_V
		6, // SEG_LVL_ALT_LF_Y_H
		6, // SEG_LVL_ALT_LF_U
		6, // SEG_LVL_ALT_LF_V
		3, // SEG_LVL_REF_FRAME
		0, // SEG_LVL_SKIP
		0, // SEG_LVL_GLOBAL_MV
	}
)

// Reference frame indices used across the spec (§3.5).
const (
	IntraFrame    = 0
	LastFrame     = 1
	Last2Frame    = 2
	Last3Frame    = 3
	GoldenFrame   = 4
	Bwdref_Frame  = 5
	Altref2_Frame = 6
	Altref_Frame  = 7
)
