package transform

import "testing"

func TestRowOpDctDct4x4(t *testing.T) {
	op := RowOp(DctDct, Tx4x4)
	if op == nil {
		t.Fatalf("RowOp(DctDct, 4x4) = nil, want IDCT4")
	}
	x := []int32{16384, 0, 0, 0}
	op(x)
	// Should produce constant output identical to IDCT4 directly.
	y := []int32{16384, 0, 0, 0}
	IDCT4(y)
	for i := range x {
		if x[i] != y[i] {
			t.Errorf("dispatch mismatch at %d: %d vs %d", i, x[i], y[i])
		}
	}
}

func TestRowOpIdentityVDct(t *testing.T) {
	// VDct: row = IDCT, col = IDTX.
	op := RowOp(VDct, Tx8x8)
	if op == nil {
		t.Fatalf("RowOp(VDct, 8x8) = nil")
	}
}

func TestColOpAdstPair(t *testing.T) {
	// AdstDct: row = DCT, col = ADST. For 4x4, col op must be IADST4.
	op := ColOp(AdstDct, Tx4x4)
	if op == nil {
		t.Fatalf("ColOp(AdstDct, 4x4) = nil")
	}
	x := []int32{100, 50, -25, 10}
	y := []int32{100, 50, -25, 10}
	op(x)
	IADST4(y)
	for i := range x {
		if x[i] != y[i] {
			t.Errorf("col op mismatch at %d: %d vs %d", i, x[i], y[i])
		}
	}
}

func TestFlipadst4Reverses(t *testing.T) {
	a := []int32{100, 50, -25, 10}
	b := []int32{100, 50, -25, 10}
	IADST4(a)
	IFLIPADST4(b)
	// b should be a reversed
	for i := 0; i < 4; i++ {
		if a[i] != b[3-i] {
			t.Errorf("IFLIPADST4[%d]=%d want %d (IADST4[%d])", i, b[3-i], a[i], i)
		}
	}
}

func TestUnsupportedReturnsNil(t *testing.T) {
	if RowOp(DctDct, Tx16x16) != nil {
		t.Errorf("RowOp(DctDct, 16x16) should be nil until IDCT16 lands")
	}
}
