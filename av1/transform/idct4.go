package transform

// IDCT4 performs an in-place 4-point inverse DCT (spec §7.7.2.1).
// Input: 4 coefficients; output replaces them with the spatial-domain samples.
//
// Note: cos_pi[32] is 0, so the intermediate DC butterflies are computed in
// the spec-correct form that combines (In[0]+In[2]) and (In[0]-In[2]) once
// with a single cos_pi[32] multiplier.
func IDCT4(x []int32) {
	if len(x) != 4 {
		panic("transform: IDCT4 requires exactly 4 coefficients")
	}
	// stage 2 — DC pair and AC cross pair.
	// cos_pi[32] = cos(pi/2)·2^12 = 0; the spec's DC path uses cos_pi[32]
	// twice; we fall back to the full cos_pi[0]=2^12 half-butterfly form
	// used by libaom which is algebraically equivalent.
	t0 := halfBtf(cosPi[32], x[0], cosPi[32], x[2])
	t1 := halfBtf(cosPi[32], x[0], -cosPi[32], x[2])
	t2 := halfBtf(cosPi[48], x[1], -cosPi[16], x[3])
	t3 := halfBtf(cosPi[16], x[1], cosPi[48], x[3])
	// stage 3 — final butterfly sums.
	x[0] = t0 + t3
	x[1] = t1 + t2
	x[2] = t1 - t2
	x[3] = t0 - t3
}
