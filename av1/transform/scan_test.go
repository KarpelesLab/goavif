package transform

import "testing"

func TestDefaultScan4x4(t *testing.T) {
	// libaom default_scan_4x4 verbatim.
	want := []int{0, 1, 4, 8, 5, 2, 3, 6, 9, 12, 13, 10, 7, 11, 14, 15}
	got := DefaultZigzagScan(4, 4)
	if len(got) != len(want) {
		t.Fatalf("length: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("scan[%d]=%d want %d", i, got[i], want[i])
		}
	}
}

func TestDefaultScanCoversAllPositions(t *testing.T) {
	for _, sz := range [][2]int{{4, 4}, {4, 8}, {8, 4}, {8, 8}, {16, 16}} {
		w, h := sz[0], sz[1]
		scan := DefaultZigzagScan(w, h)
		if len(scan) != w*h {
			t.Errorf("%dx%d: got %d entries, want %d", w, h, len(scan), w*h)
			continue
		}
		seen := make(map[int]bool)
		for _, p := range scan {
			if p < 0 || p >= w*h {
				t.Errorf("%dx%d: out-of-range %d", w, h, p)
			}
			if seen[p] {
				t.Errorf("%dx%d: duplicate %d", w, h, p)
			}
			seen[p] = true
		}
	}
}

func TestInverseScanRoundtrip(t *testing.T) {
	scan := DefaultZigzagScan(4, 4)
	iscan := InverseScan(scan)
	for i, p := range scan {
		if iscan[p] != i {
			t.Errorf("iscan[%d]=%d, want %d", p, iscan[p], i)
		}
	}
}
