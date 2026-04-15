package quant

// Plane selects which channel's quantization parameters apply.
type Plane int

const (
	PlaneY Plane = iota
	PlaneU
	PlaneV
)

// Params carries the uncompressed-header quantization deltas needed to
// compute per-plane DC/AC dequantizers.
type Params struct {
	BaseQIndex int
	DeltaQYDc  int
	DeltaQUDc  int
	DeltaQUAc  int
	DeltaQVDc  int
	DeltaQVAc  int
	BitDepth   int
}

// Values holds a pair of dequantization scales for a single plane.
type Values struct {
	DC uint16
	AC uint16
}

// Compute returns the DC and AC dequantizers for the given plane after
// applying the relevant delta and clipping to 0..255 per spec §7.12.2.6.
//
// Returns {0, 0} if the bit depth tables are not available.
func (p Params) Compute(pl Plane) Values {
	dcTable := DCLookup(p.BitDepth)
	acTable := ACLookup(p.BitDepth)
	if dcTable == nil || acTable == nil {
		return Values{}
	}
	var qDC, qAC int
	switch pl {
	case PlaneY:
		qDC = p.BaseQIndex + p.DeltaQYDc
		qAC = p.BaseQIndex
	case PlaneU:
		qDC = p.BaseQIndex + p.DeltaQUDc
		qAC = p.BaseQIndex + p.DeltaQUAc
	case PlaneV:
		qDC = p.BaseQIndex + p.DeltaQVDc
		qAC = p.BaseQIndex + p.DeltaQVAc
	}
	return Values{
		DC: dcTable[clipQ(qDC)],
		AC: acTable[clipQ(qAC)],
	}
}

func clipQ(q int) int {
	switch {
	case q < 0:
		return 0
	case q > 255:
		return 255
	}
	return q
}
