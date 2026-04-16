package transform

// IWHT4 is the 4-point inverse Walsh-Hadamard transform used by AV1's
// lossless coding path (UNIT_QUANT_SHIFT = 2 baked in). Per libaom
// reference — "4-point reversible, orthonormal inverse WHT in 3.5 adds,
// 0.5 shifts per pixel."
//
// Structure per libaom:
//
//	a = x[0] >> 2
//	c = x[1] >> 2
//	d = x[2] >> 2
//	b = x[3] >> 2
//	a += c
//	d -= b
//	e = (a - d) >> 1
//	b = e - b
//	c = e - c
//	a -= b
//	d += c
//	out = (a, b, c, d)
const unitQuantShift = 2

func IWHT4(x []int32) {
	if len(x) != 4 {
		panic("transform: IWHT4 requires exactly 4 coefficients")
	}
	a := x[0] >> unitQuantShift
	c := x[1] >> unitQuantShift
	d := x[2] >> unitQuantShift
	b := x[3] >> unitQuantShift
	a += c
	d -= b
	e := (a - d) >> 1
	b = e - b
	c = e - c
	a -= b
	d += c
	x[0] = a
	x[1] = b
	x[2] = c
	x[3] = d
}
