package obu

import "github.com/KarpelesLab/goavif/av1/bitio"

// LoopRestorationParams decodes lr_params() (spec §5.9.20).
type LoopRestorationParams struct {
	FrameRestorationType [3]uint8 // 0=NONE, 1=WIENER, 2=SGR, 3=SWITCHABLE
	UsesLR               bool
	UsesChromaLR         bool
	Log2RestorationUnitSize [3]uint8
}

const (
	RestorationNone       = 0
	RestorationWiener     = 1
	RestorationSGR        = 2 // self-guided
	RestorationSwitchable = 3
)

func parseLoopRestorationParams(r *bitio.Reader, lr *LoopRestorationParams, sh *SequenceHeader, fh *FrameHeader) {
	if fh.CodedLosslessHint() || fh.AllowIntrabc || !sh.EnableRestoration {
		for i := 0; i < 3; i++ {
			lr.FrameRestorationType[i] = RestorationNone
		}
		lr.UsesLR = false
		lr.UsesChromaLR = false
		return
	}
	lookup := [4]uint8{RestorationNone, RestorationSwitchable, RestorationWiener, RestorationSGR}
	lr.UsesLR = false
	lr.UsesChromaLR = false
	planes := sh.Color.NumPlanes
	for i := uint8(0); i < planes; i++ {
		lr.FrameRestorationType[i] = lookup[r.F(2)]
		if lr.FrameRestorationType[i] != RestorationNone {
			lr.UsesLR = true
			if i > 0 {
				lr.UsesChromaLR = true
			}
		}
	}
	if lr.UsesLR {
		if sh.Use128x128Superblock {
			lr.Log2RestorationUnitSize[0] = uint8(r.F(1)) + 7
		} else {
			lr.Log2RestorationUnitSize[0] = uint8(r.F(1)) + 6
			if lr.Log2RestorationUnitSize[0] == 6 {
				lr.Log2RestorationUnitSize[0] += uint8(r.F(1))
			}
		}
		if planes > 1 {
			if sh.Color.SubsamplingX == 1 && sh.Color.SubsamplingY == 1 && lr.UsesChromaLR {
				lr.Log2RestorationUnitSize[1] = lr.Log2RestorationUnitSize[0] - uint8(r.F(1))
			} else {
				lr.Log2RestorationUnitSize[1] = lr.Log2RestorationUnitSize[0]
			}
			lr.Log2RestorationUnitSize[2] = lr.Log2RestorationUnitSize[1]
		}
	}
}
