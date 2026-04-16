package entropy

import (
	"math/big"

	"github.com/KarpelesLab/goavif/av1/bitio"
)

// Encoder is the forward counterpart of [Decoder]. It implements a
// deferred-emission range encoder: all narrowing operations update
// the internal big-integer state, and bits are emitted only at
// [Encoder.Finish]. This avoids the carry-propagation complexity of
// a streaming range encoder at the cost of holding the running low
// value until finish; memory use is O(total bits encoded).
//
// The emitted bytes are bit-exact with goavif's [Decoder]: running
// Encoder.Init, some EncodeBool / EncodeSymbol / EncodeLiteral
// sequence, then Finish produces a byte stream that Decoder.Init
// plus the same sequence of DecodeBool / DecodeSymbol / ReadLiteral
// returns.
type Encoder struct {
	// low tracks the encoder's lower bound in the scaled interval.
	// After `shift` renormalization doublings the interval is
	// [low, low + rng) within the [0, 2^(15+shift)) space.
	low       *big.Int
	rng       uint64
	shift     int
	updateCDF bool
}

// Init resets the encoder state for a new tile.
func (e *Encoder) Init(allowCDFUpdate bool) {
	e.low = new(big.Int)
	e.rng = SymbolRange0 // 32768
	e.shift = 0
	e.updateCDF = allowCDFUpdate
}

// renormalize doubles low and rng until rng >= 32768. Each doubling
// increments the shift counter so Finish knows how many renormalize
// bits to emit.
func (e *Encoder) renormalize() {
	for e.rng < SymbolCarry {
		e.low.Lsh(e.low, 1)
		e.rng <<= 1
		e.shift++
	}
}

// EncodeBool writes a boolean whose probability of being 1 is p/32768.
// Mirrors [Decoder.DecodeBool].
func (e *Encoder) EncodeBool(bit uint32, p uint32) {
	split := (e.rng-1)*uint64(p)>>15 + MinProb
	if bit == 0 {
		e.rng = split
	} else {
		e.low.Add(e.low, new(big.Int).SetUint64(split))
		e.rng -= split
	}
	e.renormalize()
}

// EncodeSymbol writes a symbol index drawn from a CDF, mirroring
// [Decoder.DecodeSymbol]. cdf must have length N+1 where the final
// slot is the update counter. Symbol indices past N-1 are clamped.
//
// The decoder's narrowing loop has one oddity: after eliminating
// symbols 0..N-2 via the "SV >= prob_i" branch, it breaks
// implicitly with symbol=N-1 without ever looking at cdf[N-1]. The
// encoder mirrors this: for symbol == N-1 the final rng is simply
// whatever's left after eliminating the prior probs.
func (e *Encoder) EncodeSymbol(cdf []uint16, symbol int) {
	N := len(cdf) - 1
	if N < 1 || symbol < 0 || symbol >= N {
		return
	}
	r := e.rng
	loAdd := uint64(0)
	// Eliminate symbols 0..symbol-1 (decoder's "take the eliminate branch").
	for i := 0; i < symbol; i++ {
		f := uint64(cdf[i])
		factor := ((r >> 8) * (f >> ProbShift)) >> (7 - ProbShift)
		factor += uint64(MinProb * (N - i - 1))
		prob := r - factor
		loAdd += prob
		r -= prob
	}
	var newRng uint64
	if symbol == N-1 {
		// Implicit-last path: no further prob computed.
		newRng = r
	} else {
		f := uint64(cdf[symbol])
		factor := ((r >> 8) * (f >> ProbShift)) >> (7 - ProbShift)
		factor += uint64(MinProb * (N - symbol - 1))
		newRng = r - factor
	}
	e.low.Add(e.low, new(big.Int).SetUint64(loAdd))
	e.rng = newRng
	e.renormalize()
	if e.updateCDF {
		updateCDF(cdf, N, symbol)
	}
}

// EncodeLiteral writes n raw bits (50/50 bools) to the stream, MSB
// first. Inverse of [Decoder.ReadLiteral].
func (e *Encoder) EncodeLiteral(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		e.EncodeBool((v>>uint(i))&1, 16384)
	}
}

// Finish produces the serialized byte stream.
//
// At this point e.low is in [0, 2^(15+shift)). The decoder's initial
// read is 15 bits XOR'd with 0x7FFF to produce symbolValue, followed
// by 'shift' more bits read one-at-a-time during renormalization.
//
// For decoder.symbolValue to track e.low, we emit:
//   - First 15 bits = 0x7FFF XOR high15bitsOf(low)
//   - Next 'shift' bits = low's low 'shift' bits, MSB first
func (e *Encoder) Finish() []byte {
	bw := bitio.NewWriter()
	// Split e.low into high-15-bits and low-shift-bits.
	// high15 = low >> shift
	// lowShift = low & ((1 << shift) - 1)
	if e.shift < 0 {
		e.shift = 0
	}
	shift := uint(e.shift)
	high15 := new(big.Int).Rsh(e.low, shift)
	// Mask into 15-bit value.
	mask15 := new(big.Int).SetUint64((1 << 15) - 1)
	high15.And(high15, mask15)

	first15 := uint32(0x7FFF) ^ uint32(high15.Int64())
	bw.F(15, first15)

	// Emit 'shift' bits, MSB-first.
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), shift), big.NewInt(1))
	lowBits := new(big.Int).And(e.low, mask)
	for i := int(shift) - 1; i >= 0; i-- {
		bit := new(big.Int).Rsh(lowBits, uint(i))
		bit.And(bit, big.NewInt(1))
		bw.F(1, uint32(bit.Uint64()))
	}

	// Pad to byte boundary so trailing bitio reads don't overflow.
	bw.ByteAlign()
	return append([]byte(nil), bw.Bytes()...)
}

// Bytes returns the current buffer after calling Finish. Calling Bytes
// without Finish is not useful — the deferred-emission design produces
// output only at finalization.
func (e *Encoder) Bytes() []byte {
	return e.Finish()
}
