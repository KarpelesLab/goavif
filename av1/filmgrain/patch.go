package filmgrain

// Template is a rectangular block of pre-computed grain samples. The
// spec builds a 73×73 luma template (spec §7.20.3.5) shared across the
// whole frame; chroma uses a 38×38 template (§7.20.3.6). At apply time
// each 32×32 output patch looks up samples in the template at a
// position deterministically derived from (row, col) so the grain is
// stable across frames of the same sequence.
type Template struct {
	Data []int16 // Rows × Cols signed samples, row-major.
	Rows int
	Cols int
}

// NewLumaTemplate builds the 73×73 luma template (spec §7.20.3.5) with
// LFSR samples optionally shaped by an AR filter of the given lag. Pass
// lag = 0 or a nil/mismatched coeffs slice to skip the shaping step.
func NewLumaTemplate(seed uint16, lag int, coeffs []int8, shift uint8) Template {
	const rows = 73
	const cols = 73
	t := Template{
		Data: GenerateGrainTemplate(cols, rows, seed),
		Rows: rows,
		Cols: cols,
	}
	ApplyAR(t.Data, cols, rows, lag, coeffs, shift)
	return t
}

// NewChromaTemplate builds a 38×38 chroma template suitable for a
// 4:2:0 plane (spec §7.20.3.6).
func NewChromaTemplate(seed uint16, lag int, coeffs []int8, shift uint8) Template {
	const rows = 38
	const cols = 38
	t := Template{
		Data: GenerateGrainTemplate(cols, rows, seed),
		Rows: rows,
		Cols: cols,
	}
	ApplyAR(t.Data, cols, rows, lag, coeffs, shift)
	return t
}

// Sample returns the template value at (r, c); coords are wrapped into
// the template's grid.
func (t *Template) Sample(r, c int) int16 {
	if t.Rows == 0 || t.Cols == 0 {
		return 0
	}
	r = ((r % t.Rows) + t.Rows) % t.Rows
	c = ((c % t.Cols) + t.Cols) % t.Cols
	return t.Data[r*t.Cols+c]
}

// ApplyWithTemplate replaces the naive ApplyGrainPlane algorithm with a
// proper per-32×32-patch tiling. For every 32×32 block at position
// (px*32, py*32) in the output, the template offset is derived via a
// per-block LFSR seeded from (grainSeed ^ (py*37 + px*11)). Inside the
// block each sample pulls from template coordinate
// (row_offset + y, col_offset + x).
//
// This is a spec-inspired simplification — the real spec (§7.20.3.4)
// uses an extra pseudo-random hashing step that depends on the tile
// structure and frame parameters we aren't tracking yet.
func ApplyWithTemplate(plane []uint8, w, h, stride int,
	scaling *ScalingLUT, template *Template, p *Params) {
	if p == nil || p.GrainSeed == 0 || template == nil || template.Rows == 0 {
		return
	}
	shift := p.ScalingShift
	if shift == 0 {
		shift = 8
	}
	lo, hi := 0, 255
	if p.ClipToRestrictedRange {
		lo, hi = 16, 235
	}
	rng := NewRNG(0)
	for patchY := 0; patchY < h; patchY += 32 {
		for patchX := 0; patchX < w; patchX += 32 {
			seed := p.GrainSeed ^ uint16((patchY/32)*37+(patchX/32)*11)
			if seed == 0 {
				seed = 1
			}
			rng.Seed(seed)
			// Offsets modulo (template extent - 32) so the 32×32 window
			// always fits inside the template without wrapping.
			maxR := template.Rows - 32
			maxC := template.Cols - 32
			if maxR < 1 {
				maxR = 1
			}
			if maxC < 1 {
				maxC = 1
			}
			rOff := int(rng.Next()) % maxR
			cOff := int(rng.Next()) % maxC
			if rOff < 0 {
				rOff = -rOff
			}
			if cOff < 0 {
				cOff = -cOff
			}
			for dy := 0; dy < 32 && patchY+dy < h; dy++ {
				for dx := 0; dx < 32 && patchX+dx < w; dx++ {
					x := patchX + dx
					y := patchY + dy
					pix := plane[y*stride+x]
					scale := int(scaling.Lookup(pix))
					grain := int(template.Sample(rOff+dy, cOff+dx))
					delta := (grain * scale) >> uint(shift)
					v := int(pix) + delta
					if v < lo {
						v = lo
					}
					if v > hi {
						v = hi
					}
					plane[y*stride+x] = uint8(v)
				}
			}
		}
	}
}
