package quant

// QuantizeCoeff applies forward quantization to a single transform
// coefficient. Given the raw (post-FDCT) residual coefficient, its
// position (0 = DC, else AC), and the per-plane quantizers from
// [Params.Compute], it returns the signed quantized coefficient.
//
// The spec's inverse op is: raw = quant · dequantizer. So forward:
// quant = round(raw / dequantizer), with signed rounding to keep the
// round-trip sign-symmetric.
func QuantizeCoeff(raw int32, pos int, vals Values) int32 {
	var q uint16
	if pos == 0 {
		q = vals.DC
	} else {
		q = vals.AC
	}
	if q == 0 {
		return 0
	}
	half := int32(q) / 2
	if raw >= 0 {
		return (raw + half) / int32(q)
	}
	return -(((-raw) + half) / int32(q))
}

// QuantizeBlock applies [QuantizeCoeff] to every element of a
// transform block laid out in row-major order. The block is mutated
// in place.
//
// scan maps scan-position → block-position; when nil the block is
// treated as pre-scan-ordered (position 0 = DC).
func QuantizeBlock(coeffs []int32, vals Values) {
	for i := range coeffs {
		coeffs[i] = QuantizeCoeff(coeffs[i], i, vals)
	}
}
