package transform

import "testing"

func TestFDCT4RoundTripMatchesIDCT4(t *testing.T) {
	src := []int32{100, -20, 30, -5}
	cp := append([]int32(nil), src...)
	FDCT4(cp)
	IDCT4(cp)
	for i, v := range cp {
		diff := int(v) - int(src[i])
		if diff < -4 || diff > 4 {
			t.Fatalf("FDCT4→IDCT4 roundtrip mismatch at %d: got %d want %d (diff %d)", i, v, src[i], diff)
		}
	}
}

func TestFDCT8RoundTripMatchesIDCT8(t *testing.T) {
	src := []int32{100, -20, 30, -5, 15, 60, -80, 10}
	cp := append([]int32(nil), src...)
	FDCT8(cp)
	IDCT8(cp)
	for i, v := range cp {
		diff := int(v) - int(src[i])
		if diff < -8 || diff > 8 {
			t.Fatalf("FDCT8→IDCT8 roundtrip at %d: got %d want %d (diff %d)", i, v, src[i], diff)
		}
	}
}

func TestFDCT16RoundTrip(t *testing.T) {
	src := make([]int32, 16)
	for i := range src {
		src[i] = int32((i*37 + 11) % 200)
	}
	cp := append([]int32(nil), src...)
	FDCT16(cp)
	IDCT16(cp)
	for i, v := range cp {
		diff := int(v) - int(src[i])
		if diff < -16 || diff > 16 {
			t.Fatalf("FDCT16 roundtrip at %d: got %d want %d (diff %d)", i, v, src[i], diff)
		}
	}
}

func TestFDCT32RoundTrip(t *testing.T) {
	src := make([]int32, 32)
	for i := range src {
		src[i] = int32((i * i * 13) % 300)
	}
	cp := append([]int32(nil), src...)
	FDCT32(cp)
	IDCT32(cp)
	for i, v := range cp {
		diff := int(v) - int(src[i])
		if diff < -32 || diff > 32 {
			t.Fatalf("FDCT32 roundtrip at %d: got %d want %d (diff %d)", i, v, src[i], diff)
		}
	}
}

func TestFDCT64RoundTrip(t *testing.T) {
	src := make([]int32, 64)
	for i := range src {
		src[i] = int32((i * 7 & 0x3F) - 32)
	}
	cp := append([]int32(nil), src...)
	FDCT64(cp)
	IDCT64(cp)
	for i, v := range cp {
		diff := int(v) - int(src[i])
		if diff < -64 || diff > 64 {
			t.Fatalf("FDCT64 roundtrip at %d: got %d want %d (diff %d)", i, v, src[i], diff)
		}
	}
}

func TestFIdentityIsDoublingShift(t *testing.T) {
	x := []int32{1, 2, 3, 4}
	FIdentity4(x)
	if x[0] != 2 || x[3] != 8 {
		t.Fatalf("FIdentity4 not doubling: %v", x)
	}
}
