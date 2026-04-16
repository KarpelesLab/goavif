package loopfilter

// Thresholds16 carries the bit-depth-scaled threshold values for the
// 10/12-bit deblocking path. Each threshold is the 8-bit value
// left-shifted by (bitDepth - 8).
type Thresholds16 struct {
	Limit    uint16
	Blimit   uint16
	Thresh   uint16
	BitDepth int
}

// ScaleThresholds16 returns a Thresholds16 produced by left-shifting
// the 8-bit fields of th by (bitDepth - 8). bitDepth must be 8, 10, or
// 12; other values fall back to 8.
func ScaleThresholds16(th Thresholds, bitDepth int) Thresholds16 {
	if bitDepth != 10 && bitDepth != 12 {
		bitDepth = 8
	}
	shift := uint(bitDepth - 8)
	return Thresholds16{
		Limit:    uint16(th.Limit) << shift,
		Blimit:   uint16(th.Blimit) << shift,
		Thresh:   uint16(th.Thresh) << shift,
		BitDepth: bitDepth,
	}
}

// NarrowMask16 is the uint16 counterpart of [NarrowMask].
func NarrowMask16(p1, p0, q0, q1 uint16, th Thresholds16) bool {
	if absDiff16(p1, p0) > th.Blimit {
		return false
	}
	if absDiff16(q1, q0) > th.Blimit {
		return false
	}
	if absDiff16(p0, q0)*2+absDiff16(p1, q1)/2 > th.Limit {
		return false
	}
	return true
}

// HighEdgeVariation16 is the uint16 counterpart of [HighEdgeVariation].
func HighEdgeVariation16(p1, p0, q0, q1, thresh uint16) bool {
	return absDiff16(p1, p0) > thresh || absDiff16(q1, q0) > thresh
}

// Filter4_16 is the uint16 counterpart of [Filter4]. Per libaom's HBD
// path (highbd_filter4), samples are interpreted in signed form by
// subtracting the bit-depth midpoint; the clip range widens to
// ±128<<(bitDepth-8); the +4 / +3 rounding constants stay at their
// 8-bit values so that on a flat region the output is identical to
// the input.
func Filter4_16(p1, p0, q0, q1 uint16, hev bool, bitDepth int) (newP1, newP0, newQ0, newQ1 uint16) {
	shift := uint(bitDepth - 8)
	clipLimit := 128 << shift
	maxV := (1 << uint(bitDepth)) - 1

	clipS := func(v int) int {
		if v < -clipLimit {
			return -clipLimit
		}
		if v > clipLimit-1 {
			return clipLimit - 1
		}
		return v
	}
	clipU := func(v int) uint16 {
		if v < 0 {
			return 0
		}
		if v > maxV {
			return uint16(maxV)
		}
		return uint16(v)
	}

	a := clipS(int(p1) - int(q1))
	if hev {
		a = 0
	}
	b := clipS(3*(int(q0)-int(p0)) + a)
	c1 := clipS(b+4) >> 3
	c2 := clipS(b+3) >> 3

	newP0 = clipU(int(p0) + c2)
	newQ0 = clipU(int(q0) - c1)
	if !hev {
		d := (c1 + 1) >> 1
		newP1 = clipU(int(p1) + d)
		newQ1 = clipU(int(q1) - d)
	} else {
		newP1 = p1
		newQ1 = q1
	}
	return
}

// ApplyVerticalEdge4_16 is the uint16 counterpart of [ApplyVerticalEdge4].
func ApplyVerticalEdge4_16(img []uint16, stride, x, h int, th Thresholds16) {
	for r := 0; r < h; r++ {
		base := r*stride + x - 2
		p1, p0, q0, q1 := img[base], img[base+1], img[base+2], img[base+3]
		if !NarrowMask16(p1, p0, q0, q1, th) {
			continue
		}
		hev := HighEdgeVariation16(p1, p0, q0, q1, th.Thresh)
		np1, np0, nq0, nq1 := Filter4_16(p1, p0, q0, q1, hev, th.BitDepth)
		img[base] = np1
		img[base+1] = np0
		img[base+2] = nq0
		img[base+3] = nq1
	}
}

// ApplyHorizontalEdge4_16 is the uint16 counterpart of [ApplyHorizontalEdge4].
func ApplyHorizontalEdge4_16(img []uint16, stride, y, w int, th Thresholds16) {
	for c := 0; c < w; c++ {
		p1 := img[(y-2)*stride+c]
		p0 := img[(y-1)*stride+c]
		q0 := img[y*stride+c]
		q1 := img[(y+1)*stride+c]
		if !NarrowMask16(p1, p0, q0, q1, th) {
			continue
		}
		hev := HighEdgeVariation16(p1, p0, q0, q1, th.Thresh)
		np1, np0, nq0, nq1 := Filter4_16(p1, p0, q0, q1, hev, th.BitDepth)
		img[(y-2)*stride+c] = np1
		img[(y-1)*stride+c] = np0
		img[y*stride+c] = nq0
		img[(y+1)*stride+c] = nq1
	}
}

func absDiff16(a, b uint16) uint16 {
	if a > b {
		return a - b
	}
	return b - a
}
