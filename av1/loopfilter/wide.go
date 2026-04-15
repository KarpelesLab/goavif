package loopfilter

// Flat8Mask checks whether the 8-sample "flat" condition holds around an
// edge, i.e. all inner differences are within a small (×1) threshold.
// Spec §7.14.4.3.
func Flat8Mask(p3, p2, p1, p0, q0, q1, q2, q3 uint8) bool {
	const thresh = 1
	return absDiff(p1, p0) <= thresh &&
		absDiff(q1, q0) <= thresh &&
		absDiff(p2, p0) <= thresh &&
		absDiff(q2, q0) <= thresh &&
		absDiff(p3, p0) <= thresh &&
		absDiff(q3, q0) <= thresh
}

// Filter8 is the wide 8-tap deblocking filter (spec §7.14.6.3). It
// replaces 6 samples (p2..p0, q0..q2) with weighted averages of the
// surrounding 8 samples. p3 and q3 contribute to the averages but are not
// themselves updated.
//
// The weights are 1,1,1,2,1,1,1 (summing to 8) centered on each output
// position; the 8 is applied as a right-shift by 3 with a round-half-up
// bias of +4.
//
// Output order: newP2, newP1, newP0, newQ0, newQ1, newQ2.
func Filter8(p3, p2, p1, p0, q0, q1, q2, q3 uint8) (np2, np1, np0, nq0, nq1, nq2 uint8) {
	np2 = avg8(int(p3), int(p3), int(p3), 2*int(p2), int(p1), int(p0), int(q0))
	np1 = avg8(int(p3), int(p3), int(p2), 2*int(p1), int(p0), int(q0), int(q1))
	np0 = avg8(int(p3), int(p2), int(p1), 2*int(p0), int(q0), int(q1), int(q2))
	nq0 = avg8(int(p2), int(p1), int(p0), 2*int(q0), int(q1), int(q2), int(q3))
	nq1 = avg8(int(p1), int(p0), int(q0), 2*int(q1), int(q2), int(q3), int(q3))
	nq2 = avg8(int(p0), int(q0), int(q1), 2*int(q2), int(q3), int(q3), int(q3))
	return
}

// avg8 returns round((a+b+c+d+e+f+g)/8) with round-to-nearest-even via +4
// bias. Inputs are weighted-sum terms with total weight 8.
func avg8(a, b, c, d, e, f, g int) uint8 {
	s := a + b + c + d + e + f + g + 4
	return uint8(s >> 3)
}
