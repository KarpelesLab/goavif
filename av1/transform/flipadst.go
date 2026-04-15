package transform

// IFLIPADST4 is the 4-point inverse FLIPADST (spec §7.7.2.2). The forward
// transform reverses the order of the ADST outputs, so the inverse is IADST
// followed by reversing the spatial-domain result in-place.
func IFLIPADST4(x []int32) {
	if len(x) != 4 {
		panic("transform: IFLIPADST4 requires exactly 4 coefficients")
	}
	IADST4(x)
	x[0], x[3] = x[3], x[0]
	x[1], x[2] = x[2], x[1]
}
