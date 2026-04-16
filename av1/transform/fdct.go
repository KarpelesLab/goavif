package transform

// fdctMatrixInverse runs the forward N-point transform by applying
// Miᵀ·y/(2·4096) where Mi is the integer matrix AV1's IDCT butterfly
// implements. The matrix is extracted empirically by running the
// inverse transform on the scaled basis vectors e_k·4096 once per
// transform size.
//
// This produces bit-exact round-trip within the rounding of halfBtf.
// Encoders typically use a dedicated forward butterfly (libaom's
// fdct4/8/16/...), but the matrix form is short and easy to audit.
func fdctMatrixInverse(x []int32, inverse func([]int32), n int) {
	// Extract Mi by running the inverse transform on basis vectors
	// scaled by 4096 so the output fills cosPi-scale integers.
	mi := make([][]int32, n)
	for k := 0; k < n; k++ {
		basis := make([]int32, n)
		basis[k] = 1 << cosBits
		inverse(basis)
		mi[k] = basis // basis holds Mi[·][k] in its 4096-scaled form
	}
	// mi[k][i] is the coefficient of X[k] in the computation of y[i].
	// AV1's inverse transform matrix satisfies Mi·Miᵀ = (N/2)·4096²·I
	// (rows have constant L2 norm proportional to N). The algebraic
	// inverse is therefore Miᵀ·(2/N)/4096, so:
	//     X[k] = (2/N) · sum_i Mi[k][i] · y[i] / 4096
	// which we fold into a single (>>log2(N·2048)) right-shift.
	out := make([]int32, n)
	shift := uint(cosBits)
	divisor := int64(n) * int64(1<<cosBits) / 2 // = N·4096/2 = N·2048
	_ = shift                                   // shift included in divisor
	for k := 0; k < n; k++ {
		var sum int64
		for i := 0; i < n; i++ {
			sum += int64(mi[k][i]) * int64(x[i])
		}
		// Signed-round divide by divisor.
		if sum >= 0 {
			out[k] = int32((sum + divisor/2) / divisor)
		} else {
			out[k] = int32(-((-sum + divisor/2) / divisor))
		}
	}
	copy(x, out)
}

// FDCT8 performs an 8-point forward DCT. Symmetric with [IDCT8].
func FDCT8(x []int32) {
	if len(x) != 8 {
		panic("transform: FDCT8 requires exactly 8 coefficients")
	}
	fdctMatrixInverse(x, IDCT8, 8)
}

// FDCT16 performs a 16-point forward DCT.
func FDCT16(x []int32) {
	if len(x) != 16 {
		panic("transform: FDCT16 requires exactly 16 coefficients")
	}
	fdctMatrixInverse(x, IDCT16, 16)
}

// FDCT32 performs a 32-point forward DCT.
func FDCT32(x []int32) {
	if len(x) != 32 {
		panic("transform: FDCT32 requires exactly 32 coefficients")
	}
	fdctMatrixInverse(x, IDCT32, 32)
}

// FDCT64 performs a 64-point forward DCT.
func FDCT64(x []int32) {
	if len(x) != 64 {
		panic("transform: FDCT64 requires exactly 64 coefficients")
	}
	fdctMatrixInverse(x, IDCT64, 64)
}

// FIdentity4 performs a 4-point forward identity transform (scaled
// pass-through used by TX_IDTX modes).
func FIdentity4(x []int32) {
	if len(x) != 4 {
		panic("transform: FIdentity4 requires exactly 4 coefficients")
	}
	for i := range x {
		x[i] <<= 1
	}
}

// FIdentity8 performs an 8-point forward identity transform.
func FIdentity8(x []int32) {
	if len(x) != 8 {
		panic("transform: FIdentity8 requires exactly 8 coefficients")
	}
	for i := range x {
		x[i] <<= 1
	}
}

// FIdentity16 performs a 16-point forward identity transform.
func FIdentity16(x []int32) {
	if len(x) != 16 {
		panic("transform: FIdentity16 requires exactly 16 coefficients")
	}
	for i := range x {
		x[i] <<= 1
	}
}

// FIdentity32 performs a 32-point forward identity transform.
func FIdentity32(x []int32) {
	if len(x) != 32 {
		panic("transform: FIdentity32 requires exactly 32 coefficients")
	}
	for i := range x {
		x[i] <<= 1
	}
}
