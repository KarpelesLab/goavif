package lr

// Plane describes a single reconstructed plane for loop restoration.
type Plane struct {
	Pix    []uint8
	Stride int
	Width  int
	Height int
}

// FilterType identifies which LR filter to apply to a restoration unit.
type FilterType uint8

const (
	FilterNone   FilterType = 0
	FilterWiener FilterType = 1
	FilterSGR    FilterType = 2
)

// UnitParams holds the per-restoration-unit filter choice and its
// coefficients. A caller decoding from a real bitstream fills this
// struct from the unit's signaled values; tests or synthetic drivers
// can populate it directly.
type UnitParams struct {
	Filter FilterType
	// Wiener: horizontal + vertical tap sets.
	WienerHoriz WienerTaps
	WienerVert  WienerTaps
	// SGR params.
	SGR SGRParams
}

// UnitFn returns the per-unit parameters for a restoration unit at
// plane-relative (unitX, unitY) coordinates (top-left of the unit).
type UnitFn func(unitX, unitY int) UnitParams

// ApplyFrame walks unitSize × unitSize restoration units across the
// plane, calling fn for each to get the filter choice and running
// that filter into the same plane.
//
// The AV1 spec signals restoration units at 64 / 128 / 256 sample
// sizes (controlled by Log2RestorationUnitSize); this helper takes
// unitSize directly so it can match the frame header's setting.
//
// For tile edges and frame edges the spec inserts 3-sample-wide
// "strip" buffers before / after filter reads; this simpler driver
// operates on unit boundaries aligned with the plane's implicit
// clamp-at-edge behavior — close enough for the AVIF still path.
func ApplyFrame(p Plane, unitSize int, fn UnitFn) {
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
				unitSrc := sliceUnit(p.Pix, p.Stride, x, y, uw, uh)
				unitDst := make([]uint8, uw*uh)
				ApplyWiener(unitDst, unitSrc, uw, uh, uw, params.WienerHoriz, params.WienerVert)
				writeUnit(p.Pix, p.Stride, x, y, uw, uh, unitDst)
			case FilterSGR:
				unitSrc := sliceUnit(p.Pix, p.Stride, x, y, uw, uh)
				unitDst := make([]uint8, uw*uh)
				ApplySGR(unitDst, unitSrc, uw, uh, uw, params.SGR)
				writeUnit(p.Pix, p.Stride, x, y, uw, uh, unitDst)
			}
		}
	}
}

// sliceUnit copies a uw×uh rectangle starting at (x, y) into a flat
// row-major slice.
func sliceUnit(pix []uint8, stride, x, y, uw, uh int) []uint8 {
	out := make([]uint8, uw*uh)
	for r := 0; r < uh; r++ {
		copy(out[r*uw:(r+1)*uw], pix[(y+r)*stride+x:(y+r)*stride+x+uw])
	}
	return out
}

// writeUnit writes a uw×uh flat slice back into a stride-indexed plane.
func writeUnit(pix []uint8, stride, x, y, uw, uh int, src []uint8) {
	for r := 0; r < uh; r++ {
		copy(pix[(y+r)*stride+x:(y+r)*stride+x+uw], src[r*uw:(r+1)*uw])
	}
}
