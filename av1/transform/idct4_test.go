package transform

import "testing"

func TestIDCT4RoundtripDC(t *testing.T) {
	// The forward DCT of a constant 8,8,8,8 signal produces a single DC
	// coefficient and zeros elsewhere. Running the inverse should reproduce
	// the constant (up to the AV1 convention that the inverse transform is
	// applied to dequantized coefficients scaled to the spatial range).
	//
	// For a 4-point orthonormal DCT, the DC coefficient of [c,c,c,c] equals
	// 2c (with cos(pi/4)=sqrt(1/2)); AV1's transform constants scale by
	// cosPi[32] = 2^12, so we compare scaled results.
	in := []int32{32768, 0, 0, 0}
	IDCT4(in)
	// All four outputs should be equal (DC reconstruction).
	for i := 1; i < 4; i++ {
		if in[i] != in[0] {
			t.Errorf("sample[%d]=%d != sample[0]=%d", i, in[i], in[0])
		}
	}
	if in[0] == 0 {
		t.Errorf("DC reconstruction is zero")
	}
}

func TestIDCT4Linearity(t *testing.T) {
	// IDCT4 is a linear operator: IDCT4(a+b) = IDCT4(a) + IDCT4(b).
	a := []int32{100, 50, -25, 10}
	b := []int32{-30, 40, 5, -12}
	sum := []int32{a[0] + b[0], a[1] + b[1], a[2] + b[2], a[3] + b[3]}

	aCopy := append([]int32(nil), a...)
	bCopy := append([]int32(nil), b...)
	IDCT4(aCopy)
	IDCT4(bCopy)
	IDCT4(sum)
	for i := 0; i < 4; i++ {
		want := aCopy[i] + bCopy[i]
		// Allow ±1 ulp of rounding slack from the two separate butterflies.
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			t.Errorf("linearity violated at i=%d: got %d, want %d", i, sum[i], want)
		}
	}
}
