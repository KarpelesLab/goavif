package transform

import "sync"

// fdctMatrixInverse runs the forward N-point transform by applying
// Miᵀ·y/(N·2048) where Mi is the integer matrix AV1's IDCT butterfly
// implements. Mi is computed by running the inverse transform on
// scaled basis vectors e_k·2^cosBits. Per-size matrices are cached
// globally via [miFor], eliminating the N+1 allocations that used
// to dominate encoder profiles.
//
// This produces bit-exact round-trip within the rounding of halfBtf.
// Encoders typically use a dedicated forward butterfly (libaom's
// fdct4/8/16/...), but the matrix form is short and easy to audit.
func fdctMatrixInverse(x []int32, inverse func([]int32), n int) {
	mi := miFor(inverse, n)
	// mi[k][i] is the coefficient of X[k] in the computation of y[i].
	// AV1's inverse transform matrix satisfies Mi·Miᵀ = (N/2)·4096²·I
	// (rows have constant L2 norm proportional to N). The algebraic
	// inverse is therefore Miᵀ·(2/N)/4096, so:
	//     X[k] = (2/N) · sum_i Mi[k][i] · y[i] / 4096
	// which we fold into a single (>>log2(N·2048)) right-shift.
	divisor := int64(n) * int64(1<<cosBits) / 2 // = N·4096/2 = N·2048
	half := divisor / 2
	// Overwrite x in-place via a stack-capable scratch of at most
	// 64 int32s (we never exceed 64-point DCT).
	var scratch [64]int32
	out := scratch[:n]
	for k := 0; k < n; k++ {
		var sum int64
		row := mi[k]
		for i := 0; i < n; i++ {
			sum += int64(row[i]) * int64(x[i])
		}
		if sum >= 0 {
			out[k] = int32((sum + half) / divisor)
		} else {
			out[k] = int32(-((-sum + half) / divisor))
		}
	}
	copy(x, out)
}

var (
	miOnce [5]sync.Once    // 4, 8, 16, 32, 64
	miMats [5][][]int32    // cached matrices
)

// miFor returns the precomputed Mi matrix for the given 1-D inverse.
// Matrices are keyed by log2(n)-2 so lookup is O(1). Other N values
// fall through to a per-call computation (none occur in practice).
func miFor(inverse func([]int32), n int) [][]int32 {
	idx := -1
	switch n {
	case 4:
		idx = 0
	case 8:
		idx = 1
	case 16:
		idx = 2
	case 32:
		idx = 3
	case 64:
		idx = 4
	}
	if idx < 0 {
		return buildMi(inverse, n)
	}
	miOnce[idx].Do(func() {
		miMats[idx] = buildMi(inverse, n)
	})
	return miMats[idx]
}

func buildMi(inverse func([]int32), n int) [][]int32 {
	mi := make([][]int32, n)
	for k := 0; k < n; k++ {
		basis := make([]int32, n)
		basis[k] = 1 << cosBits
		inverse(basis)
		mi[k] = basis
	}
	return mi
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
