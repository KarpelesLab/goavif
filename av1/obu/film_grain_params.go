package obu

import "github.com/KarpelesLab/goavif/av1/bitio"

// FilmGrainParams decodes film_grain_params() (spec §5.9.30).
type FilmGrainParams struct {
	ApplyGrain     bool
	GrainSeed      uint16
	UpdateGrain    bool
	FilmGrainRef   uint8

	NumYPoints     uint8
	PointYValue    [14]uint8
	PointYScaling  [14]uint8

	ChromaScaling bool
	NumCbPoints   uint8
	PointCbValue  [10]uint8
	PointCbScaling [10]uint8
	NumCrPoints   uint8
	PointCrValue  [10]uint8
	PointCrScaling [10]uint8

	GrainScalingMinus8 uint8
	ARCoeffLag         uint8
	ARCoeffsY          [24]int8
	ARCoeffsCb         [25]int8
	ARCoeffsCr         [25]int8
	ARCoeffShiftMinus6 uint8

	GrainScaleShift   uint8
	CbMult            uint8
	CbLumaMult        uint8
	CbOffset          uint16
	CrMult            uint8
	CrLumaMult        uint8
	CrOffset          uint16

	OverlapFlag         bool
	ClipToRestrictedRange bool
}

func parseFilmGrainParams(r *bitio.Reader, g *FilmGrainParams, sh *SequenceHeader, fh *FrameHeader) {
	if !sh.FilmGrainParamsPresent || (!fh.ShowFrame && !fh.ShowableFrame) {
		return
	}
	g.ApplyGrain = r.F(1) == 1
	if !g.ApplyGrain {
		return
	}
	g.GrainSeed = uint16(r.F(16))
	if fh.FrameType == InterFrame {
		g.UpdateGrain = r.F(1) == 1
	} else {
		g.UpdateGrain = true
	}
	if !g.UpdateGrain {
		g.FilmGrainRef = uint8(r.F(3))
		return
	}
	g.NumYPoints = uint8(r.F(4))
	for i := uint8(0); i < g.NumYPoints; i++ {
		g.PointYValue[i] = uint8(r.F(8))
		g.PointYScaling[i] = uint8(r.F(8))
	}
	if sh.Color.Monochrome {
		g.ChromaScaling = false
	} else if sh.Color.SubsamplingX == 1 && sh.Color.SubsamplingY == 1 && g.NumYPoints == 0 {
		g.ChromaScaling = false
	} else {
		g.ChromaScaling = r.F(1) == 1
	}
	if !sh.Color.Monochrome && !(sh.Color.SubsamplingX == 1 && sh.Color.SubsamplingY == 1 && g.NumYPoints == 0) {
		g.NumCbPoints = uint8(r.F(4))
		for i := uint8(0); i < g.NumCbPoints; i++ {
			g.PointCbValue[i] = uint8(r.F(8))
			g.PointCbScaling[i] = uint8(r.F(8))
		}
		g.NumCrPoints = uint8(r.F(4))
		for i := uint8(0); i < g.NumCrPoints; i++ {
			g.PointCrValue[i] = uint8(r.F(8))
			g.PointCrScaling[i] = uint8(r.F(8))
		}
	}
	g.GrainScalingMinus8 = uint8(r.F(2))
	g.ARCoeffLag = uint8(r.F(2))
	numPosY := 2 * int(g.ARCoeffLag) * (int(g.ARCoeffLag) + 1)
	numPosChroma := numPosY
	if g.NumYPoints > 0 {
		numPosChroma++
	}
	if g.NumYPoints > 0 {
		for i := 0; i < numPosY; i++ {
			g.ARCoeffsY[i] = int8(int(r.F(8)) - 128)
		}
	}
	if g.ChromaScaling || g.NumCbPoints > 0 {
		for i := 0; i < numPosChroma; i++ {
			g.ARCoeffsCb[i] = int8(int(r.F(8)) - 128)
		}
	}
	if g.ChromaScaling || g.NumCrPoints > 0 {
		for i := 0; i < numPosChroma; i++ {
			g.ARCoeffsCr[i] = int8(int(r.F(8)) - 128)
		}
	}
	g.ARCoeffShiftMinus6 = uint8(r.F(2))
	g.GrainScaleShift = uint8(r.F(2))
	if g.NumCbPoints > 0 {
		g.CbMult = uint8(r.F(8))
		g.CbLumaMult = uint8(r.F(8))
		g.CbOffset = uint16(r.F(9))
	}
	if g.NumCrPoints > 0 {
		g.CrMult = uint8(r.F(8))
		g.CrLumaMult = uint8(r.F(8))
		g.CrOffset = uint16(r.F(9))
	}
	g.OverlapFlag = r.F(1) == 1
	g.ClipToRestrictedRange = r.F(1) == 1
}
