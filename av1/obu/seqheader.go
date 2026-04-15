package obu

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/bitio"
)

// SELECT_SCREEN_CONTENT_TOOLS and SELECT_INTEGER_MV are sentinels defined by
// the spec to indicate the encoder did not force the tool choice.
const (
	SelectScreenContentTools = 2
	SelectIntegerMV          = 2
)

// Spec constants for color config defaults (CICP unspecified).
const (
	CPUnspecified uint8 = 2
	TCUnspecified uint8 = 2
	MCUnspecified uint8 = 2

	CPBT709    uint8 = 1
	TCSRGB     uint8 = 13
	MCIdentity uint8 = 0
)

// ColorConfig carries the color-specific fields of the sequence header
// (spec §5.5.2). Some fields are derived, not directly coded:
//
//   - BitDepth is computed from (seq_profile, high_bitdepth, twelve_bit).
//   - NumPlanes is 1 when monochrome else 3.
type ColorConfig struct {
	BitDepth                    uint8
	NumPlanes                   uint8
	Monochrome                  bool
	ColorDescriptionPresentFlag bool
	ColorPrimaries              uint8
	TransferCharacteristics     uint8
	MatrixCoefficients          uint8
	ColorRange                  bool
	SubsamplingX                uint8
	SubsamplingY                uint8
	ChromaSamplePosition        uint8
	SeparateUVDeltaQ            bool
}

// OperatingPoint encapsulates one of N operating points declared by the
// sequence header.
type OperatingPoint struct {
	IDC                                  uint16
	SeqLevelIdx                          uint8
	SeqTier                              uint8
	DecoderModelPresentForThisOP         bool
	InitialDisplayDelayPresentForThisOP  bool
	InitialDisplayDelayMinusOne          uint8
}

// TimingInfo is present when timing_info_present_flag is set (spec §5.5.3).
type TimingInfo struct {
	NumUnitsInDisplayTick uint32
	TimeScale             uint32
	EqualPictureInterval  bool
	NumTicksPerPictureMinusOne uint32
}

// DecoderModelInfo — spec §5.5.4. Collected verbatim for round-trip but
// unused by the still-image decode path.
type DecoderModelInfo struct {
	BufferDelayLengthMinusOne        uint8
	NumUnitsInDecodingTick           uint32
	BufferRemovalTimeLengthMinusOne  uint8
	FramePresentationTimeLengthMinusOne uint8
}

// SequenceHeader is the decoded OBU_SEQUENCE_HEADER (spec §5.5).
type SequenceHeader struct {
	SeqProfile                   uint8
	StillPicture                 bool
	ReducedStillPictureHeader    bool

	TimingInfoPresentFlag          bool
	TimingInfo                     TimingInfo
	DecoderModelInfoPresentFlag    bool
	DecoderModelInfo               DecoderModelInfo
	InitialDisplayDelayPresentFlag bool

	OperatingPoints []OperatingPoint

	FrameWidthBitsMinusOne  uint8
	FrameHeightBitsMinusOne uint8
	MaxFrameWidthMinusOne   uint32
	MaxFrameHeightMinusOne  uint32

	FrameIDNumbersPresentFlag bool
	DeltaFrameIDLengthMinusTwo   uint8
	AdditionalFrameIDLengthMinusOne uint8

	Use128x128Superblock   bool
	EnableFilterIntra      bool
	EnableIntraEdgeFilter  bool
	EnableInterintraCompound bool
	EnableMaskedCompound   bool
	EnableWarpedMotion     bool
	EnableDualFilter       bool
	EnableOrderHint        bool
	EnableJntComp          bool
	EnableRefFrameMvs      bool

	SeqForceScreenContentTools uint8 // 0, 1, or SelectScreenContentTools
	SeqForceIntegerMV          uint8 // 0, 1, or SelectIntegerMV

	OrderHintBitsMinusOne uint8

	EnableSuperres    bool
	EnableCdef        bool
	EnableRestoration bool

	Color ColorConfig

	FilmGrainParamsPresent bool
}

// ParseSequenceHeader decodes an OBU_SEQUENCE_HEADER payload. The payload
// is the bytes of the OBU after the header, matching [OBU.Payload] for an
// OBU of type [TypeSequenceHeader].
func ParseSequenceHeader(payload []byte) (*SequenceHeader, error) {
	r := bitio.NewReader(payload)
	sh := &SequenceHeader{}

	sh.SeqProfile = uint8(r.F(3))
	sh.StillPicture = r.F(1) == 1
	if sh.StillPicture {
		sh.ReducedStillPictureHeader = r.F(1) == 1
	} else {
		if r.F(1) != 0 {
			return nil, fmt.Errorf("%w: reduced_still_picture_header set without still_picture", ErrMalformed)
		}
	}

	if sh.ReducedStillPictureHeader {
		op := OperatingPoint{
			IDC:         0,
			SeqLevelIdx: uint8(r.F(5)),
		}
		sh.OperatingPoints = []OperatingPoint{op}
	} else {
		sh.TimingInfoPresentFlag = r.F(1) == 1
		if sh.TimingInfoPresentFlag {
			sh.TimingInfo.NumUnitsInDisplayTick = uint32(r.F(32))
			sh.TimingInfo.TimeScale = uint32(r.F(32))
			sh.TimingInfo.EqualPictureInterval = r.F(1) == 1
			if sh.TimingInfo.EqualPictureInterval {
				sh.TimingInfo.NumTicksPerPictureMinusOne = r.Uvlc()
			}
			sh.DecoderModelInfoPresentFlag = r.F(1) == 1
			if sh.DecoderModelInfoPresentFlag {
				sh.DecoderModelInfo.BufferDelayLengthMinusOne = uint8(r.F(5))
				sh.DecoderModelInfo.NumUnitsInDecodingTick = uint32(r.F(32))
				sh.DecoderModelInfo.BufferRemovalTimeLengthMinusOne = uint8(r.F(5))
				sh.DecoderModelInfo.FramePresentationTimeLengthMinusOne = uint8(r.F(5))
			}
		}
		sh.InitialDisplayDelayPresentFlag = r.F(1) == 1
		opCount := uint8(r.F(5)) + 1
		sh.OperatingPoints = make([]OperatingPoint, opCount)
		for i := uint8(0); i < opCount; i++ {
			op := &sh.OperatingPoints[i]
			op.IDC = uint16(r.F(12))
			op.SeqLevelIdx = uint8(r.F(5))
			if op.SeqLevelIdx > 7 {
				op.SeqTier = uint8(r.F(1))
			}
			if sh.DecoderModelInfoPresentFlag {
				op.DecoderModelPresentForThisOP = r.F(1) == 1
				if op.DecoderModelPresentForThisOP {
					// operating_parameters_info — two LEB128-sized fields;
					// we consume but do not store them.
					nBits := uint(sh.DecoderModelInfo.BufferDelayLengthMinusOne) + 1
					_ = r.F64(nBits) // decoder_buffer_delay
					_ = r.F64(nBits) // encoder_buffer_delay
					_ = r.F(1)       // low_delay_mode_flag
				}
			}
			if sh.InitialDisplayDelayPresentFlag {
				op.InitialDisplayDelayPresentForThisOP = r.F(1) == 1
				if op.InitialDisplayDelayPresentForThisOP {
					op.InitialDisplayDelayMinusOne = uint8(r.F(4))
				}
			}
		}
	}

	sh.FrameWidthBitsMinusOne = uint8(r.F(4))
	sh.FrameHeightBitsMinusOne = uint8(r.F(4))
	sh.MaxFrameWidthMinusOne = uint32(r.F(uint(sh.FrameWidthBitsMinusOne) + 1))
	sh.MaxFrameHeightMinusOne = uint32(r.F(uint(sh.FrameHeightBitsMinusOne) + 1))

	if !sh.ReducedStillPictureHeader {
		sh.FrameIDNumbersPresentFlag = r.F(1) == 1
		if sh.FrameIDNumbersPresentFlag {
			sh.DeltaFrameIDLengthMinusTwo = uint8(r.F(4))
			sh.AdditionalFrameIDLengthMinusOne = uint8(r.F(3))
		}
	}

	sh.Use128x128Superblock = r.F(1) == 1
	sh.EnableFilterIntra = r.F(1) == 1
	sh.EnableIntraEdgeFilter = r.F(1) == 1

	if !sh.ReducedStillPictureHeader {
		sh.EnableInterintraCompound = r.F(1) == 1
		sh.EnableMaskedCompound = r.F(1) == 1
		sh.EnableWarpedMotion = r.F(1) == 1
		sh.EnableDualFilter = r.F(1) == 1
		sh.EnableOrderHint = r.F(1) == 1
		if sh.EnableOrderHint {
			sh.EnableJntComp = r.F(1) == 1
			sh.EnableRefFrameMvs = r.F(1) == 1
		}
		if r.F(1) == 1 { // seq_choose_screen_content_tools
			sh.SeqForceScreenContentTools = SelectScreenContentTools
		} else {
			sh.SeqForceScreenContentTools = uint8(r.F(1))
		}
		if sh.SeqForceScreenContentTools > 0 {
			if r.F(1) == 1 { // seq_choose_integer_mv
				sh.SeqForceIntegerMV = SelectIntegerMV
			} else {
				sh.SeqForceIntegerMV = uint8(r.F(1))
			}
		} else {
			sh.SeqForceIntegerMV = SelectIntegerMV
		}
		if sh.EnableOrderHint {
			sh.OrderHintBitsMinusOne = uint8(r.F(3))
		}
	} else {
		sh.SeqForceScreenContentTools = SelectScreenContentTools
		sh.SeqForceIntegerMV = SelectIntegerMV
	}

	sh.EnableSuperres = r.F(1) == 1
	sh.EnableCdef = r.F(1) == 1
	sh.EnableRestoration = r.F(1) == 1

	if err := parseColorConfig(r, sh); err != nil {
		return nil, err
	}
	sh.FilmGrainParamsPresent = r.F(1) == 1

	if err := r.TrailingBits(); err != nil {
		return nil, fmt.Errorf("%w: trailing bits: %w", ErrMalformed, err)
	}
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return sh, nil
}

// parseColorConfig decodes the color_config() block (spec §5.5.2) into
// sh.Color. It handles the sRGB-identity special case and all profile-
// specific subsampling rules.
func parseColorConfig(r *bitio.Reader, sh *SequenceHeader) error {
	cc := &sh.Color
	highBit := r.F(1) == 1
	twelveBit := false
	if sh.SeqProfile == 2 && highBit {
		twelveBit = r.F(1) == 1
	}
	switch {
	case twelveBit:
		cc.BitDepth = 12
	case highBit:
		cc.BitDepth = 10
	default:
		cc.BitDepth = 8
	}

	if sh.SeqProfile == 1 {
		cc.Monochrome = false
	} else {
		cc.Monochrome = r.F(1) == 1
	}
	if cc.Monochrome {
		cc.NumPlanes = 1
	} else {
		cc.NumPlanes = 3
	}

	cc.ColorDescriptionPresentFlag = r.F(1) == 1
	if cc.ColorDescriptionPresentFlag {
		cc.ColorPrimaries = uint8(r.F(8))
		cc.TransferCharacteristics = uint8(r.F(8))
		cc.MatrixCoefficients = uint8(r.F(8))
	} else {
		cc.ColorPrimaries = CPUnspecified
		cc.TransferCharacteristics = TCUnspecified
		cc.MatrixCoefficients = MCUnspecified
	}

	if cc.Monochrome {
		cc.ColorRange = r.F(1) == 1
		cc.SubsamplingX = 1
		cc.SubsamplingY = 1
		cc.ChromaSamplePosition = 0
		cc.SeparateUVDeltaQ = false
		return nil
	}

	// sRGB identity special case.
	if cc.ColorPrimaries == CPBT709 &&
		cc.TransferCharacteristics == TCSRGB &&
		cc.MatrixCoefficients == MCIdentity {
		cc.ColorRange = true
		cc.SubsamplingX = 0
		cc.SubsamplingY = 0
	} else {
		cc.ColorRange = r.F(1) == 1
		switch sh.SeqProfile {
		case 0:
			cc.SubsamplingX = 1
			cc.SubsamplingY = 1
		case 1:
			cc.SubsamplingX = 0
			cc.SubsamplingY = 0
		case 2:
			if cc.BitDepth == 12 {
				cc.SubsamplingX = uint8(r.F(1))
				if cc.SubsamplingX == 1 {
					cc.SubsamplingY = uint8(r.F(1))
				}
			} else {
				cc.SubsamplingX = 1
				cc.SubsamplingY = 0
			}
		}
		if cc.SubsamplingX == 1 && cc.SubsamplingY == 1 {
			cc.ChromaSamplePosition = uint8(r.F(2))
		}
	}
	cc.SeparateUVDeltaQ = r.F(1) == 1
	return nil
}
