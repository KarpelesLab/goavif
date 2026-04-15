package transform

// DefaultZigzagScan generates the AV1 default scan order for a w×h
// transform block (spec §7.9.2.1 default_scan_*). Each entry of the
// returned slice is the row-major position of the next coefficient in
// scan order.
//
// The pattern alternates by anti-diagonal:
//
//	sum=0  (rows desc / cols asc)  → just (0,0)
//	sum=1  (rows asc  / cols desc) → (0,1), (1,0)
//	sum=2  (rows desc / cols asc)  → (2,0), (1,1), (0,2)
//	…
//
// For 4×4 the output matches libaom's default_scan_4x4 verbatim.
func DefaultZigzagScan(w, h int) []int {
	n := w * h
	out := make([]int, 0, n)
	maxSum := w + h - 2
	for sum := 0; sum <= maxSum; sum++ {
		if sum%2 == 0 {
			// even diagonals: rows descending, cols ascending
			r := sum
			c := 0
			if r >= h {
				c += r - (h - 1)
				r = h - 1
			}
			for r >= 0 && c < w {
				out = append(out, r*w+c)
				r--
				c++
			}
		} else {
			// odd diagonals: rows ascending, cols descending
			c := sum
			r := 0
			if c >= w {
				r += c - (w - 1)
				c = w - 1
			}
			for c >= 0 && r < h {
				out = append(out, r*w+c)
				r++
				c--
			}
		}
	}
	return out
}

// InverseScan inverts a scan order: given scan[i] = block position of
// the i'th visited coefficient, returns iscan[pos] = the scan-order
// index of the coefficient at block position pos.
func InverseScan(scan []int) []int {
	n := len(scan)
	out := make([]int, n)
	for i, p := range scan {
		out[p] = i
	}
	return out
}
