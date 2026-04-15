package obu

import "github.com/KarpelesLab/goavif/av1/bitio"

// LoopFilterParams decodes loop_filter_params() (spec §5.9.11).
type LoopFilterParams struct {
	LevelY0           uint8
	LevelY1           uint8
	LevelU            uint8
	LevelV            uint8
	Sharpness         uint8
	ModeRefDeltaEnabled bool
	ModeRefDeltaUpdate  bool
	RefDeltas         [TotalRefsPerFrame]int8
	ModeDeltas        [2]int8
}

// defaultRefDeltas per spec §7.12.1.
var defaultRefDeltas = [TotalRefsPerFrame]int8{1, 0, 0, 0, -1, 0, -1, -1}

// parseLoopFilterParams consumes loop filter fields.
func parseLoopFilterParams(r *bitio.Reader, lf *LoopFilterParams, fh *FrameHeader, sh *SequenceHeader) {
	// Initialize with defaults first so skipped cases produce correct state.
	lf.RefDeltas = defaultRefDeltas
	lf.ModeDeltas = [2]int8{0, 0}

	if fh.CodedLosslessHint() || fh.AllowIntrabc {
		lf.LevelY0, lf.LevelY1, lf.LevelU, lf.LevelV = 0, 0, 0, 0
		lf.Sharpness = 0
		lf.ModeRefDeltaEnabled = true
		return
	}

	lf.LevelY0 = uint8(r.F(6))
	lf.LevelY1 = uint8(r.F(6))
	if sh.Color.NumPlanes > 1 {
		if lf.LevelY0 != 0 || lf.LevelY1 != 0 {
			lf.LevelU = uint8(r.F(6))
			lf.LevelV = uint8(r.F(6))
		}
	}
	lf.Sharpness = uint8(r.F(3))
	lf.ModeRefDeltaEnabled = r.F(1) == 1
	if lf.ModeRefDeltaEnabled {
		lf.ModeRefDeltaUpdate = r.F(1) == 1
		if lf.ModeRefDeltaUpdate {
			for i := 0; i < TotalRefsPerFrame; i++ {
				if r.F(1) == 1 {
					lf.RefDeltas[i] = int8(r.Su(7))
				}
			}
			for i := 0; i < 2; i++ {
				if r.F(1) == 1 {
					lf.ModeDeltas[i] = int8(r.Su(7))
				}
			}
		}
	}
}

// CodedLosslessHint mirrors the spec's CodedLossless derivation, simplified
// for the cases reachable from a still-image AVIF.
func (fh *FrameHeader) CodedLosslessHint() bool {
	if fh.Quant.BaseQIndex != 0 ||
		fh.Quant.DeltaQYDc != 0 ||
		fh.Quant.DeltaQUDc != 0 ||
		fh.Quant.DeltaQUAc != 0 ||
		fh.Quant.DeltaQVDc != 0 ||
		fh.Quant.DeltaQVAc != 0 {
		return false
	}
	return true
}
