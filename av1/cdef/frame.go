package cdef

// Plane carries a single reconstructed plane plus its stride.
type Plane struct {
	Pix    []uint8
	Stride int
	Width  int
	Height int
}

// ApplyFrame runs CDEF over an entire plane by splitting it into 8×8
// blocks, running FindDirection on each, then applying FilterBlock
// with the supplied strengths and damping.
//
// priStrength / secStrength / damping are the spec's per-superblock
// cdef-idx-resolved values. When priStrength == 0 && secStrength == 0
// the filter is skipped (no-op).
//
// The filter reads one sample past the block edge in each direction;
// out-of-range reads are clamped by [FilterBlock].
func ApplyFrame(p Plane, priStrength, secStrength, damping int) {
	if priStrength == 0 && secStrength == 0 {
		return
	}
	// CDEF operates on a temporary copy so that within-frame read/write
	// overlaps don't cascade.
	buf := make([]uint8, len(p.Pix))
	copy(buf, p.Pix)
	for y := 0; y+8 <= p.Height; y += 8 {
		for x := 0; x+8 <= p.Width; x += 8 {
			dir, _ := FindDirection(buf, p.Stride, x, y)
			FilterBlock(p.Pix, buf, p.Stride, x, y, dir,
				priStrength, secStrength, damping)
		}
	}
}
