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

// StrengthFn resolves per-superblock (priStrength, secStrength) for the
// 8×8 block whose top-left sample sits at (x, y) in plane coordinates.
// Return (0, 0) to skip the block entirely. Used by [ApplyFramePerSB]
// to route cdef_idx into distinct strengths for different SBs.
type StrengthFn func(x, y int) (pri, sec int)

// ApplyFramePerSB runs CDEF per 8×8 block using strengthFn to resolve
// strengths for that block's containing 64×64 superblock. Damping is
// frame-global (per spec §7.15). When strengthFn returns (0, 0) the
// block is skipped.
func ApplyFramePerSB(p Plane, strengthFn StrengthFn, damping int) {
	if strengthFn == nil {
		return
	}
	// Snapshot the plane so read-after-write ordering matches the spec.
	buf := make([]uint8, len(p.Pix))
	copy(buf, p.Pix)
	for y := 0; y+8 <= p.Height; y += 8 {
		for x := 0; x+8 <= p.Width; x += 8 {
			pri, sec := strengthFn(x, y)
			if pri == 0 && sec == 0 {
				continue
			}
			dir, _ := FindDirection(buf, p.Stride, x, y)
			FilterBlock(p.Pix, buf, p.Stride, x, y, dir, pri, sec, damping)
		}
	}
}
