package filmgrain

// ApplyWithTemplate16 is the uint16 counterpart of [ApplyWithTemplate].
// Output is clipped to [0, (1<<bitDepth)-1] with optional broadcast-
// legal clipping to [16<<shift, 235<<shift] for luma when requested.
//
// The scaling LUT still indexes on the high 8 bits of the sample value
// (matching the spec's LookupHighBD behaviour) so the same 256-entry
// LUT can drive any bit depth.
func ApplyWithTemplate16(plane []uint16, w, h, stride int,
	scaling *ScalingLUT, template *Template, p *Params, bitDepth int) {
	if p == nil || p.GrainSeed == 0 || template == nil || template.Rows == 0 {
		return
	}
	shift := p.ScalingShift
	if shift == 0 {
		shift = 8
	}
	maxV := (1 << uint(bitDepth)) - 1
	loY, hiY := 0, maxV
	if p.ClipToRestrictedRange {
		bdShift := uint(bitDepth - 8)
		loY = 16 << bdShift
		hiY = 235 << bdShift
	}
	rng := NewRNG(0)
	for patchY := 0; patchY < h; patchY += 32 {
		for patchX := 0; patchX < w; patchX += 32 {
			seed := p.GrainSeed ^ uint16((patchY/32)*37+(patchX/32)*11)
			if seed == 0 {
				seed = 1
			}
			rng.Seed(seed)
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
					scale := int(scaling.LookupHighBD(pix, bitDepth))
					grain := int(template.Sample(rOff+dy, cOff+dx))
					delta := (grain * scale) >> uint(shift)
					v := int(pix) + delta
					if v < loY {
						v = loY
					}
					if v > hiY {
						v = hiY
					}
					plane[y*stride+x] = uint16(v)
				}
			}
		}
	}
}
