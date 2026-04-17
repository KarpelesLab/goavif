package decoder

import (
	"sync"

	"github.com/KarpelesLab/goavif/av1/predict"
)

var mcPad16Pool = sync.Pool{
	New: func() any {
		b := make([]uint16, 71*71)
		return &b
	},
}

// MotionCompensate16 is the HBD counterpart of [MotionCompensate]: it
// produces a w×h uint16 predicted block at dst using refY at position
// (bx + mv.Col, by + mv.Row). MVs are eighth-pel units; bit depth is
// 10 or 12. The reference is edge-clamped so out-of-frame MVs are
// legal.
func MotionCompensate16(
	dst []uint16, w, h int,
	refY []uint16, refW, refH, refStride int,
	bx, by int,
	mv MV,
	filt predict.InterpFilter,
	bitDepth int,
) {
	intX := int(mv.Col) >> 3
	intY := int(mv.Row) >> 3
	phaseX := int(mv.Col) & 7
	phaseY := int(mv.Row) & 7
	hp := phaseX * 2
	vp := phaseY * 2

	if hp == 0 && vp == 0 {
		for r := 0; r < h; r++ {
			sy := clampInt(by+intY+r, 0, refH-1)
			for c := 0; c < w; c++ {
				sx := clampInt(bx+intX+c, 0, refW-1)
				dst[r*w+c] = refY[sy*refStride+sx]
			}
		}
		return
	}

	padStride := w + 7
	padLen := padStride * (h + 7)
	padPtr := mcPad16Pool.Get().(*[]uint16)
	pad := *padPtr
	if cap(pad) < padLen {
		pad = make([]uint16, padLen)
	} else {
		pad = pad[:padLen]
	}
	defer func() {
		*padPtr = pad
		mcPad16Pool.Put(padPtr)
	}()
	for r := 0; r < h+7; r++ {
		sy := clampInt(by+intY+r-3, 0, refH-1)
		for c := 0; c < w+7; c++ {
			sx := clampInt(bx+intX+c-3, 0, refW-1)
			pad[r*padStride+c] = refY[sy*refStride+sx]
		}
	}
	predict.InterpSubPel16(dst, w, h, pad, padStride, hp, vp, filt, bitDepth)
}
