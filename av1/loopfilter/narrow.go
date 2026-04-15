package loopfilter

// Thresholds carries the three threshold values used by the deblocking
// filter mask logic (spec §7.14.4). All values are expressed for 8-bit
// depth and must be scaled for 10/12-bit inputs.
type Thresholds struct {
	Limit uint8 // "limit": allowed local variation across the edge
	Blimit uint8 // "blimit": allowed inner-sample variation on each side
	Thresh uint8 // "thresh": high edge variation threshold (HEV detection)
}

// NarrowMask reports whether the 4-tap narrow filter should be applied
// given the four samples straddling an edge (p1, p0 | q0, q1).
//
// See spec §7.14.4 filter_mask:
//
//	mask  = |p1-p0|<=blimit AND |q1-q0|<=blimit AND |p0-q0|*2 + |p1-q1|/2 <= limit
func NarrowMask(p1, p0, q0, q1 uint8, th Thresholds) bool {
	if absDiff(p1, p0) > th.Blimit {
		return false
	}
	if absDiff(q1, q0) > th.Blimit {
		return false
	}
	if absDiff(p0, q0)*2+absDiff(p1, q1)/2 > th.Limit {
		return false
	}
	return true
}

// HighEdgeVariation mirrors spec §7.14.5 "hev": true when either inner
// difference (|p1-p0|, |q1-q0|) exceeds thresh, indicating a real edge for
// which only the two inner samples are adjusted.
func HighEdgeVariation(p1, p0, q0, q1 uint8, thresh uint8) bool {
	return absDiff(p1, p0) > thresh || absDiff(q1, q0) > thresh
}

// Filter4 applies the AV1 4-tap narrow deblocking filter to four samples
// across an edge (spec §7.14.6.2). Inputs and outputs are the adjusted
// p1, p0, q0, q1 values.
//
// The caller must have already verified the mask via [NarrowMask].
func Filter4(p1, p0, q0, q1 uint8, hev bool) (newP1, newP0, newQ0, newQ1 uint8) {
	a := clipS8(int(p1) - int(q1))
	if hev {
		a = 0
	}
	b := clipS8(3*(int(q0)-int(p0)) + int(a))
	c1 := clipS8(int(b) + 4) >> 3
	c2 := clipS8(int(b) + 3) >> 3

	newP0 = clipU8(int(p0) + int(c2))
	newQ0 = clipU8(int(q0) - int(c1))
	if !hev {
		d := (int(c1) + 1) >> 1
		newP1 = clipU8(int(p1) + d)
		newQ1 = clipU8(int(q1) - d)
	} else {
		newP1 = p1
		newQ1 = q1
	}
	return
}

// ApplyVerticalEdge4 applies Filter4 to every row of a vertical edge at
// column x of stride samples (w pixels wide by h pixels tall). The filter
// reads/writes img[r*stride + x-2 : r*stride + x+2] at each row r.
//
// No-op on rows where the mask rejects the filter.
func ApplyVerticalEdge4(img []uint8, stride, x, h int, th Thresholds) {
	for r := 0; r < h; r++ {
		base := r*stride + x - 2
		p1, p0, q0, q1 := img[base], img[base+1], img[base+2], img[base+3]
		if !NarrowMask(p1, p0, q0, q1, th) {
			continue
		}
		hev := HighEdgeVariation(p1, p0, q0, q1, th.Thresh)
		np1, np0, nq0, nq1 := Filter4(p1, p0, q0, q1, hev)
		img[base] = np1
		img[base+1] = np0
		img[base+2] = nq0
		img[base+3] = nq1
	}
}

// ApplyHorizontalEdge4 applies Filter4 to every column of a horizontal
// edge at row y. The filter reads/writes img[(y-2..y+1)*stride + c] at
// each column c in [0, w).
func ApplyHorizontalEdge4(img []uint8, stride, y, w int, th Thresholds) {
	for c := 0; c < w; c++ {
		p1 := img[(y-2)*stride+c]
		p0 := img[(y-1)*stride+c]
		q0 := img[y*stride+c]
		q1 := img[(y+1)*stride+c]
		if !NarrowMask(p1, p0, q0, q1, th) {
			continue
		}
		hev := HighEdgeVariation(p1, p0, q0, q1, th.Thresh)
		np1, np0, nq0, nq1 := Filter4(p1, p0, q0, q1, hev)
		img[(y-2)*stride+c] = np1
		img[(y-1)*stride+c] = np0
		img[y*stride+c] = nq0
		img[(y+1)*stride+c] = nq1
	}
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func clipS8(v int) int8 {
	switch {
	case v < -128:
		return -128
	case v > 127:
		return 127
	}
	return int8(v)
}

func clipU8(v int) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	}
	return uint8(v)
}
