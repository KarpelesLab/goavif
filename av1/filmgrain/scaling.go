package filmgrain

// ScalingLUT maps luma (or chroma) values to a scaling factor for the
// grain sample at that intensity. AV1 transmits up to 14 (value, scale)
// control points per plane; decoders expand them into a 256-entry LUT by
// piecewise-linear interpolation between consecutive points. Values
// below the first point clamp to that point's scale; values above the
// last point clamp to the last point's scale.
type ScalingLUT [256]uint8

// Point describes one (value, scale) control point from
// film_grain_params. Value is the luma level at which the scale applies
// (0..255 for 8-bit, 0..1023 for 10-bit); Scale is the multiplier
// applied to the grain sample before it's added to the pixel.
type Point struct {
	Value uint8
	Scale uint8
}

// BuildLUT expands a sorted list of control points into a full 256-entry
// scaling lookup table. Points must be sorted by Value ascending; the
// function returns a zero LUT when there are no points.
//
// This routine is written for 8-bit samples. For 10/12-bit the spec
// indexes the LUT with (sample >> (bitDepth-8)) — see [LookupHighBD].
func BuildLUT(points []Point) ScalingLUT {
	var lut ScalingLUT
	if len(points) == 0 {
		return lut
	}
	// Values below the first point clamp to its scale.
	first := points[0]
	for i := 0; i < int(first.Value); i++ {
		lut[i] = first.Scale
	}
	// Piecewise-linear between consecutive points.
	for p := 0; p < len(points)-1; p++ {
		lo := points[p]
		hi := points[p+1]
		if hi.Value <= lo.Value {
			continue
		}
		span := int(hi.Value) - int(lo.Value)
		for i := int(lo.Value); i <= int(hi.Value); i++ {
			// Linear interpolation rounded to nearest.
			d := i - int(lo.Value)
			scale := int(lo.Scale)*(span-d) + int(hi.Scale)*d
			lut[i] = uint8((scale + span/2) / span)
		}
	}
	// Values above the last point clamp to its scale.
	last := points[len(points)-1]
	for i := int(last.Value) + 1; i < 256; i++ {
		lut[i] = last.Scale
	}
	// Exact-value entries: the piecewise-linear loop above covers
	// multi-point boundaries, but with a single control point the
	// exact value slot (e.g. lut[128] when Value=128) is never
	// written, leaving it zero. Set it explicitly here.
	for _, p := range points {
		lut[p.Value] = p.Scale
	}
	return lut
}

// Lookup returns the scale factor for an 8-bit sample.
func (lut *ScalingLUT) Lookup(sample uint8) uint8 {
	return lut[sample]
}

// LookupHighBD returns the scale factor for a 10- or 12-bit sample by
// right-shifting the sample so the top 8 bits index the LUT. bitDepth
// must be 8, 10 or 12; other values produce undefined results.
func (lut *ScalingLUT) LookupHighBD(sample uint16, bitDepth int) uint8 {
	shift := bitDepth - 8
	if shift < 0 {
		shift = 0
	}
	return lut[sample>>uint(shift)]
}
