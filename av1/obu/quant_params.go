package obu

import "github.com/KarpelesLab/goavif/av1/bitio"

// QuantizationParams decodes quantization_params() (spec §5.9.12).
type QuantizationParams struct {
	BaseQIndex    uint8
	DeltaQYDc     int8
	DiffUVDelta   bool
	DeltaQUDc     int8
	DeltaQUAc     int8
	DeltaQVDc     int8
	DeltaQVAc     int8
	UsingQMatrix  bool
	QmY           uint8
	QmU           uint8
	QmV           uint8
}

func parseQuantizationParams(r *bitio.Reader, q *QuantizationParams, sh *SequenceHeader) {
	q.BaseQIndex = uint8(r.F(8))
	q.DeltaQYDc = readDeltaQ(r)
	if sh.Color.NumPlanes > 1 {
		if sh.Color.SeparateUVDeltaQ {
			q.DiffUVDelta = r.F(1) == 1
		}
		q.DeltaQUDc = readDeltaQ(r)
		q.DeltaQUAc = readDeltaQ(r)
		if q.DiffUVDelta {
			q.DeltaQVDc = readDeltaQ(r)
			q.DeltaQVAc = readDeltaQ(r)
		} else {
			q.DeltaQVDc = q.DeltaQUDc
			q.DeltaQVAc = q.DeltaQUAc
		}
	}
	q.UsingQMatrix = r.F(1) == 1
	if q.UsingQMatrix {
		q.QmY = uint8(r.F(4))
		q.QmU = uint8(r.F(4))
		if sh.Color.SeparateUVDeltaQ {
			q.QmV = uint8(r.F(4))
		} else {
			q.QmV = q.QmU
		}
	}
}

// readDeltaQ reads delta_q (spec §5.9.12): 1-bit flag then a 7-bit signed
// magnitude if present. Returns 0 when absent.
func readDeltaQ(r *bitio.Reader) int8 {
	if r.F(1) != 1 {
		return 0
	}
	return int8(r.Su(7))
}
