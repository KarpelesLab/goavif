package loopfilter

import "testing"

func TestNarrowMaskOnFlatEdge(t *testing.T) {
	// No variation at all — mask should pass.
	th := Thresholds{Limit: 20, Blimit: 10, Thresh: 8}
	if !NarrowMask(100, 100, 100, 100, th) {
		t.Errorf("flat edge should be masked")
	}
}

func TestNarrowMaskRejectsBigJump(t *testing.T) {
	th := Thresholds{Limit: 20, Blimit: 10, Thresh: 8}
	// p0 and q0 differ a lot.
	if NarrowMask(100, 100, 200, 200, th) {
		t.Errorf("large jump should not be masked")
	}
}

func TestFilter4ReducesBlockingEdge(t *testing.T) {
	// Classic block edge: left side flat at 110, right side flat at 120.
	// The narrow filter should pull p0 up and q0 down.
	th := Thresholds{Limit: 30, Blimit: 10, Thresh: 8}
	p1, p0, q0, q1 := uint8(110), uint8(110), uint8(120), uint8(120)
	if !NarrowMask(p1, p0, q0, q1, th) {
		t.Fatalf("mask rejected the edge; widen thresholds for test")
	}
	hev := HighEdgeVariation(p1, p0, q0, q1, th.Thresh)
	np1, np0, nq0, nq1 := Filter4(p1, p0, q0, q1, hev)
	if np0 <= p0 {
		t.Errorf("p0 not adjusted upward: %d -> %d", p0, np0)
	}
	if nq0 >= q0 {
		t.Errorf("q0 not adjusted downward: %d -> %d", q0, nq0)
	}
	// With !hev, p1/q1 also move; with hev they stay. Spot-check a run.
	if !hev && (np1 == p1 || nq1 == q1) {
		t.Errorf("non-HEV filter should also adjust inner samples (p1=%d->%d q1=%d->%d)", p1, np1, q1, nq1)
	}
}

func TestFilter4PreservesTrueEdge(t *testing.T) {
	// Inner variation exceeds thresh → HEV is true; in HEV mode the filter
	// only adjusts p0/q0, leaving p1/q1 untouched.
	th := Thresholds{Limit: 100, Blimit: 40, Thresh: 15}
	p1, p0, q0, q1 := uint8(20), uint8(40), uint8(60), uint8(80)
	if !NarrowMask(p1, p0, q0, q1, th) {
		t.Fatalf("test setup: mask rejected")
	}
	hev := HighEdgeVariation(p1, p0, q0, q1, th.Thresh)
	if !hev {
		t.Fatalf("expected HEV=true (inner diffs %d/%d > thresh %d)",
			absDiff(p1, p0), absDiff(q1, q0), th.Thresh)
	}
	np1, _, _, nq1 := Filter4(p1, p0, q0, q1, hev)
	if np1 != p1 || nq1 != q1 {
		t.Errorf("HEV mode changed p1/q1: %d->%d, %d->%d", p1, np1, q1, nq1)
	}
}

func TestApplyVerticalEdge4(t *testing.T) {
	// 4-column wide image, height 2, edge at x=2.
	img := []uint8{
		110, 110, 120, 120,
		110, 110, 120, 120,
	}
	ApplyVerticalEdge4(img, 4, 2, 2, Thresholds{Limit: 30, Blimit: 10, Thresh: 8})
	// The inner samples (col 1 and col 2) should have moved toward each other.
	if img[1] <= 110 || img[2] >= 120 {
		t.Errorf("edge not softened: row0 = %v", img[:4])
	}
}
