package transform

// IADST4 performs an in-place 4-point inverse ADST (spec §7.7.2.3).
//
// The 4-point ADST uses sin_pi constants rather than cos_pi; sin_pi_k_9 is
// round(sin(k*pi/9) * 2^cosBits), with cosBits = 12.
//
// Inputs are 32-bit to accommodate the intermediate multiplications before
// the final rounding.
func IADST4(x []int32) {
	if len(x) != 4 {
		panic("transform: IADST4 requires exactly 4 coefficients")
	}
	// Compute intermediate products (§7.7.2.3).
	s0 := int32(sinPi19) * x[0]
	s1 := int32(sinPi29) * x[0]
	s2 := int32(sinPi39) * x[1]
	s3 := int32(sinPi49) * x[2]
	s4 := int32(sinPi19) * x[2]
	s5 := int32(sinPi29) * x[3]
	s6 := int32(sinPi49) * x[3]
	s7 := x[0] - x[2] + x[3]

	s0 = s0 + s3 + s5
	s1 = s1 - s4 - s6
	s3 = s2
	s2 = int32(sinPi39) * s7

	x[0] = round2(s0+s3, cosBits)
	x[1] = round2(s1+s3, cosBits)
	x[2] = round2(s2, cosBits)
	x[3] = round2(s0+s1-s3, cosBits)
}
