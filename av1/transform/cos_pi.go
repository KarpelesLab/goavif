package transform

import "math"

// AV1 transform cosine constants (spec §7.7.1): cos_pi[k] = round(cos(k*pi/128) * 2^cosBits)
// for k in [0, 64]. cosBits is 12.
//
// A few landmark values:
//
//	cos_pi[0]  = 4096
//	cos_pi[8]  = round(cos(pi/16) * 4096) = 4017
//	cos_pi[16] = round(cos(pi/8)  * 4096) = 3784
//	cos_pi[32] = round(cos(pi/4)  * 4096) = 2896
//	cos_pi[48] = round(cos(3pi/8) * 4096) = 1567
//	cos_pi[56] = round(cos(7pi/16)* 4096) = 799
//	cos_pi[64] = 0
const cosBits = 12

var cosPi [65]int32

// AV1 sin_pi constants used by ADST-4 (spec §7.7.2.3):
//
//	sin_pi_k_9 = round(sin(k*pi/9) * 2^cosBits), k = 1..4
const (
	sinPi19 = 1321
	sinPi29 = 2482
	sinPi39 = 3344
	sinPi49 = 3803
)

func init() {
	for i := range cosPi {
		cosPi[i] = int32(math.Round(math.Cos(float64(i)*math.Pi/128.0) * (1 << cosBits)))
	}
}

// halfBtf computes round((w0*in0 + w1*in1) / 2^cosBits) with symmetric
// rounding, as defined in spec §7.7.1.2.
func halfBtf(w0, in0, w1, in1 int32) int32 {
	return (w0*in0 + w1*in1 + (1 << (cosBits - 1))) >> cosBits
}

// round2 implements the spec's round2(x, shift) primitive used by ADST and
// some IDCT stages: (x + (1 << (shift-1))) >> shift.
func round2(x int32, shift uint) int32 {
	if shift == 0 {
		return x
	}
	return (x + int32(1<<(shift-1))) >> shift
}
