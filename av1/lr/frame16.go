package lr

// Plane16 is the uint16 counterpart of [Plane].
type Plane16 struct {
	Pix    []uint16
	Stride int
	Width  int
	Height int
}

// ApplyFrame16 is the uint16 counterpart of [ApplyFrame]. bitDepth
// (10 or 12) is passed through to the filter primitives.
func ApplyFrame16(p Plane16, unitSize int, fn UnitFn, bitDepth int) {
	if unitSize <= 0 {
		return
	}
	for y := 0; y < p.Height; y += unitSize {
		uh := unitSize
		if y+uh > p.Height {
			uh = p.Height - y
		}
		for x := 0; x < p.Width; x += unitSize {
			uw := unitSize
			if x+uw > p.Width {
				uw = p.Width - x
			}
			params := fn(x, y)
			switch params.Filter {
			case FilterNone:
				continue
			case FilterWiener:
				unitSrc := sliceUnit16(p.Pix, p.Stride, x, y, uw, uh)
				unitDst := make([]uint16, uw*uh)
				ApplyWiener16(unitDst, unitSrc, uw, uh, uw, params.WienerHoriz, params.WienerVert, bitDepth)
				writeUnit16(p.Pix, p.Stride, x, y, uw, uh, unitDst)
			case FilterSGR:
				unitSrc := sliceUnit16(p.Pix, p.Stride, x, y, uw, uh)
				unitDst := make([]uint16, uw*uh)
				ApplySGR16(unitDst, unitSrc, uw, uh, uw, params.SGR, bitDepth)
				writeUnit16(p.Pix, p.Stride, x, y, uw, uh, unitDst)
			}
		}
	}
}

func sliceUnit16(pix []uint16, stride, x, y, uw, uh int) []uint16 {
	out := make([]uint16, uw*uh)
	for r := 0; r < uh; r++ {
		copy(out[r*uw:(r+1)*uw], pix[(y+r)*stride+x:(y+r)*stride+x+uw])
	}
	return out
}

func writeUnit16(pix []uint16, stride, x, y, uw, uh int, src []uint16) {
	for r := 0; r < uh; r++ {
		copy(pix[(y+r)*stride+x:(y+r)*stride+x+uw], src[r*uw:(r+1)*uw])
	}
}
