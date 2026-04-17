package decoder

import (
	"sync"

	"github.com/KarpelesLab/goavif/av1/predict"
)

// mcPadPool reuses the (w+7)×(h+7) padded reference region across
// sub-pel MC calls. The biggest block we generate prediction for is
// 64×64 → pad 71×71 = 5041 bytes; we size the pool's default slice
// to 64×64+7×7 = 4096+49 = 5041 to fit comfortably.
var mcPadPool = sync.Pool{
	New: func() any {
		b := make([]uint8, 71*71)
		return &b
	},
}

// MotionCompensate produces an w×h predicted block at dst using the
// reference plane `refY` at position (bx + mv.Col, by + mv.Row) where
// MV components are in eighth-pel units. The reference is clipped to
// plane bounds with edge-repeat so MVs that point past the frame
// still produce valid samples.
//
// filt selects the 8-tap interpolation filter set (REGULAR / SMOOTH /
// SHARP). When hp and vp are both zero the integer-pel fast path
// (direct copy) is used.
func MotionCompensate(
	dst []uint8, w, h int,
	refY []uint8, refW, refH, refStride int,
	bx, by int,
	mv MV,
	filt predict.InterpFilter,
) {
	// MV is eighth-pel. Split into integer pel + phase.
	intX := int(mv.Col) >> 3
	intY := int(mv.Row) >> 3
	phaseX := int(mv.Col) & 7
	phaseY := int(mv.Row) & 7
	// Spec maps an 8-tap filter across 16 phases, but only even
	// phases are used unless allow_high_precision_mv fires. We
	// map phaseX / phaseY (0..7 eighth-pel) to 16-phase indices
	// (0, 2, 4, ..., 14). When sub-pel is unused (integer-pel MV),
	// both phases are 0 and InterpInteger handles the copy.
	hp := phaseX * 2
	vp := phaseY * 2

	if hp == 0 && vp == 0 {
		// Integer-pel fast path.
		for r := 0; r < h; r++ {
			sy := by + intY + r
			sy = clampInt(sy, 0, refH-1)
			for c := 0; c < w; c++ {
				sx := bx + intX + c
				sx = clampInt(sx, 0, refW-1)
				dst[r*w+c] = refY[sy*refStride+sx]
			}
		}
		return
	}

	// Sub-pel: build a (w+7) × (h+7) padded source region with
	// clamping, then run the 8-tap filter. Reuse a pooled buffer to
	// keep per-call allocations off the hot path.
	padStride := w + 7
	padLen := padStride * (h + 7)
	padPtr := mcPadPool.Get().(*[]uint8)
	pad := *padPtr
	if cap(pad) < padLen {
		pad = make([]uint8, padLen)
	} else {
		pad = pad[:padLen]
	}
	defer func() {
		*padPtr = pad
		mcPadPool.Put(padPtr)
	}()
	for r := 0; r < h+7; r++ {
		sy := by + intY + r - 3
		sy = clampInt(sy, 0, refH-1)
		for c := 0; c < w+7; c++ {
			sx := bx + intX + c - 3
			sx = clampInt(sx, 0, refW-1)
			pad[r*padStride+c] = refY[sy*refStride+sx]
		}
	}
	predict.InterpSubPel(dst, w, h, pad, padStride, hp, vp, filt)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
