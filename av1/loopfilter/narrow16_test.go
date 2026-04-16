package loopfilter

import "testing"

func TestScaleThresholds16ShiftsForBitDepth(t *testing.T) {
	th := Thresholds{Limit: 10, Blimit: 5, Thresh: 3}
	t10 := ScaleThresholds16(th, 10)
	if t10.Limit != 40 || t10.Blimit != 20 || t10.Thresh != 12 || t10.BitDepth != 10 {
		t.Fatalf("10-bit scale wrong: %+v", t10)
	}
	t12 := ScaleThresholds16(th, 12)
	if t12.Limit != 160 || t12.Blimit != 80 || t12.Thresh != 48 || t12.BitDepth != 12 {
		t.Fatalf("12-bit scale wrong: %+v", t12)
	}
	t8 := ScaleThresholds16(th, 8)
	if t8.Limit != 10 || t8.BitDepth != 8 {
		t.Fatalf("8-bit fallthrough wrong: %+v", t8)
	}
	// Invalid bit depth falls back to 8.
	tBad := ScaleThresholds16(th, 13)
	if tBad.BitDepth != 8 {
		t.Fatalf("invalid bit depth should fall back to 8, got %d", tBad.BitDepth)
	}
}

func TestNarrowMask16RejectsLargeDelta(t *testing.T) {
	th := Thresholds16{Limit: 40, Blimit: 20, Thresh: 12, BitDepth: 10}
	if NarrowMask16(500, 700, 300, 400, th) {
		t.Fatal("NarrowMask16 should reject huge inner-sample jumps")
	}
}

func TestNarrowMask16AcceptsSmoothEdge(t *testing.T) {
	// Smooth linear ramp across the edge — all inner differences are 1.
	th := Thresholds16{Limit: 40, Blimit: 20, Thresh: 12, BitDepth: 10}
	if !NarrowMask16(500, 501, 502, 503, th) {
		t.Fatal("NarrowMask16 should accept smooth ramp")
	}
}

func TestFilter4_16SmoothsEdge(t *testing.T) {
	// A step edge: p1=p0=300, q0=q1=500 (step of 200 at 10-bit).
	p1, p0, q0, q1 := uint16(300), uint16(300), uint16(500), uint16(500)
	np1, np0, nq0, nq1 := Filter4_16(p1, p0, q0, q1, false, 10)
	// The filter should pull p0 up and q0 down.
	if np0 <= p0 {
		t.Fatalf("p0 %d -> %d should increase", p0, np0)
	}
	if nq0 >= q0 {
		t.Fatalf("q0 %d -> %d should decrease", q0, nq0)
	}
	// Without hev both outer samples should also move toward the edge.
	if np1 <= p1 {
		t.Fatalf("p1 %d -> %d should increase", p1, np1)
	}
	if nq1 >= q1 {
		t.Fatalf("q1 %d -> %d should decrease", q1, nq1)
	}
}

func TestFilter4_16ClipsToBitDepth(t *testing.T) {
	// Near-max samples so output could overshoot 4095.
	p1, p0, q0, q1 := uint16(4080), uint16(4090), uint16(0), uint16(0)
	np1, np0, nq0, nq1 := Filter4_16(p1, p0, q0, q1, false, 12)
	for _, v := range []uint16{np1, np0, nq0, nq1} {
		if v > 4095 {
			t.Fatalf("output %d exceeds 12-bit range", v)
		}
	}
}

func TestApplyFrameNarrow16IdentityOnFlatPlane(t *testing.T) {
	const W, H = 16, 16
	pix := make([]uint16, W*H)
	for i := range pix {
		pix[i] = 500
	}
	orig := append([]uint16(nil), pix...)
	th := ScaleThresholds16(Thresholds{Limit: 40, Blimit: 20, Thresh: 12}, 10)
	ApplyFrameNarrow16(Plane16{Pix: pix, Stride: W, Width: W, Height: H},
		UniformGrid(W, H, 4, 4), th)
	for i := range pix {
		if pix[i] != orig[i] {
			t.Fatalf("flat plane modified at %d: %d -> %d", i, orig[i], pix[i])
		}
	}
}
