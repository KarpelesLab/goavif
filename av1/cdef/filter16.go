package cdef

// FilterBlock16 is the uint16 counterpart of [FilterBlock]. The
// algorithm matches the uint8 variant bit-for-bit; only the sample
// type and the clip maximum differ (output is clipped to
// (1<<bitDepth)-1).
//
// CDEF's constrain() nonlinearity naturally scales with the input
// range because priStrength/secStrength are applied to the sample-
// value difference d = n - x0 in the same number of bits as the
// source. Callers supplying 10/12-bit strengths from the frame
// header can use this directly.
func FilterBlock16(dst, src []uint16, stride, x, y, dir, priStrength, secStrength, damping, bitDepth int) {
	const bs = 8
	dirOff := Directions[dir]
	secDirA := Directions[(dir+2)%8]
	secDirB := Directions[(dir+6)%8]
	maxV := (1 << uint(bitDepth)) - 1

	for r := 0; r < bs; r++ {
		for c := 0; c < bs; c++ {
			x0 := int(src[(y+r)*stride+(x+c)])
			sum := 0
			for i := 0; i < 2; i++ {
				for s := -1; s <= 1; s += 2 {
					nr := y + r + s*dirOff[i][0]
					nc := x + c + s*dirOff[i][1]
					n := sampleClamped16(src, stride, nc, nr, bitDepth)
					d := n - x0
					sum += PrimaryTaps[i] * Constrain(d, priStrength, damping)
				}
			}
			for _, so := range [2][2][2]int{secDirA, secDirB} {
				for i := 0; i < 2; i++ {
					for s := -1; s <= 1; s += 2 {
						nr := y + r + s*so[i][0]
						nc := x + c + s*so[i][1]
						n := sampleClamped16(src, stride, nc, nr, bitDepth)
						d := n - x0
						sum += SecondaryTaps[i] * Constrain(d, secStrength, damping)
					}
				}
			}
			out := x0 + (8+sum-b2i(sum < 0))>>4
			if out < 0 {
				out = 0
			} else if out > maxV {
				out = maxV
			}
			dst[(y+r)*stride+(x+c)] = uint16(out)
		}
	}
}

func sampleClamped16(src []uint16, stride, col, row, bitDepth int) int {
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	idx := row*stride + col
	if idx < 0 || idx >= len(src) {
		return 1 << uint(bitDepth-1)
	}
	return int(src[idx])
}

// Plane16 is the uint16 counterpart of [Plane].
type Plane16 struct {
	Pix    []uint16
	Stride int
	Width  int
	Height int
}

// ApplyFrame16 runs CDEF over an entire uint16 plane using the same
// algorithm as [ApplyFrame] but with bit-depth-aware clipping.
func ApplyFrame16(p Plane16, priStrength, secStrength, damping, bitDepth int) {
	if priStrength == 0 && secStrength == 0 {
		return
	}
	buf := make([]uint16, len(p.Pix))
	copy(buf, p.Pix)
	for y := 0; y+8 <= p.Height; y += 8 {
		for x := 0; x+8 <= p.Width; x += 8 {
			dir, _ := FindDirection16(buf, p.Stride, x, y, bitDepth)
			FilterBlock16(p.Pix, buf, p.Stride, x, y, dir,
				priStrength, secStrength, damping, bitDepth)
		}
	}
}

// StrengthFn16 is the uint16 counterpart of [StrengthFn].
type StrengthFn16 func(x, y int) (pri, sec int)

// ApplyFramePerSB16 is the uint16 counterpart of [ApplyFramePerSB].
func ApplyFramePerSB16(p Plane16, strengthFn StrengthFn16, damping, bitDepth int) {
	if strengthFn == nil {
		return
	}
	buf := make([]uint16, len(p.Pix))
	copy(buf, p.Pix)
	for y := 0; y+8 <= p.Height; y += 8 {
		for x := 0; x+8 <= p.Width; x += 8 {
			pri, sec := strengthFn(x, y)
			if pri == 0 && sec == 0 {
				continue
			}
			dir, _ := FindDirection16(buf, p.Stride, x, y, bitDepth)
			FilterBlock16(p.Pix, buf, p.Stride, x, y, dir, pri, sec, damping, bitDepth)
		}
	}
}
