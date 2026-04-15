package loopfilter

import "testing"

func TestFlat8MaskConstant(t *testing.T) {
	if !Flat8Mask(100, 100, 100, 100, 100, 100, 100, 100) {
		t.Errorf("constant block should be flat")
	}
	// Flat8Mask checks *per-side* flatness (|pX-p0|, |qX-q0|). A flat-100
	// and flat-110 block is thus still flat; cross-edge rejection happens
	// via NarrowMask upstream.
	if !Flat8Mask(100, 100, 100, 100, 110, 110, 110, 110) {
		t.Errorf("two flat sides should pass Flat8Mask even with a cross-edge jump")
	}
	// A block with noise on one side must fail.
	if Flat8Mask(100, 100, 100, 100, 110, 115, 120, 125) {
		t.Errorf("non-flat q side should fail Flat8Mask")
	}
}

func TestFilter8ConstantIdentity(t *testing.T) {
	p2, p1, p0, q0, q1, q2 := Filter8(50, 50, 50, 50, 50, 50, 50, 50)
	for i, v := range []uint8{p2, p1, p0, q0, q1, q2} {
		if v != 50 {
			t.Errorf("constant input Filter8[%d]=%d want 50", i, v)
		}
	}
}

func TestFilter8SoftensEdge(t *testing.T) {
	// A block edge: all samples <50 on one side, all >50 on the other.
	p2, p1, p0, q0, q1, q2 := Filter8(40, 40, 40, 40, 60, 60, 60, 60)
	// The filter should pull p0 up toward the middle and q0 down toward it.
	if p0 <= 40 || q0 >= 60 {
		t.Errorf("edge not softened: p0=%d q0=%d", p0, q0)
	}
	// p2 and q2 should move only slightly.
	if absDiff(p2, 40) > 8 {
		t.Errorf("p2 moved too much: %d vs 40", p2)
	}
	if absDiff(q2, 60) > 8 {
		t.Errorf("q2 moved too much: %d vs 60", q2)
	}
	// Sanity: p1 between p2 and p0, q1 between q0 and q2.
	if !(p2 <= p1 && p1 <= p0) {
		t.Errorf("p-side not monotonic: p2=%d p1=%d p0=%d", p2, p1, p0)
	}
	if !(q0 <= q1 && q1 <= q2) {
		t.Errorf("q-side not monotonic: q0=%d q1=%d q2=%d", q0, q1, q2)
	}
}
