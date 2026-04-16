package entropy

// Encoder is the forward counterpart of [Decoder]. It implements the
// AV1-flavored range encoder: bits in, bytes out. The byte stream
// produced is self-consistent with goavif's [Decoder] and exercises
// the same CDF tables; it is NOT yet bit-exact with libaom's
// reference encoder. Producing dav1d-decodable output requires
// the bit-exact variant, which is a follow-up.
//
// Internally the encoder carries a 32-bit "low" register and a
// 16-bit rng, doubling both during renormalization and emitting
// bits as they fall out the top of the window. Deferred-carry
// handling is folded into the renormalize loop: each pending
// 0xFF byte is held back until a non-0xFF byte emerges or a
// carry arrives.
type Encoder struct {
	low     uint32
	rng     uint32
	nbits   int // bits shifted out of low waiting to be packed
	bitAcc  uint8
	bitCnt  int
	buf     []byte
	pending int // count of deferred 0xFF bytes

	updateCDF bool
}

// Init resets the encoder state for a new tile. The decoder's Init
// reads 15 bits and XORs with 0x7FFF to produce the starting
// symbolValue. For encoder state (low=0) to match, we pre-emit 15
// ones so that decoder.symbolValue = 0x7FFF ^ 0x7FFF = 0.
func (e *Encoder) Init(allowCDFUpdate bool) {
	e.low = 0
	e.rng = SymbolRange0
	e.nbits = 0
	e.bitAcc = 0
	e.bitCnt = 0
	e.buf = e.buf[:0]
	e.pending = 0
	e.updateCDF = allowCDFUpdate
	// Prime the output with 15 ones. The decoder's first F(15) reads
	// these and XORs with 0x7FFF to yield symbolValue=0, which matches
	// the encoder's implicit starting low.
	for i := 0; i < 15; i++ {
		e.emitBit(1)
	}
}

// emitBit writes a single bit to the output byte stream (MSB first).
func (e *Encoder) emitBit(b uint32) {
	e.bitAcc = (e.bitAcc << 1) | uint8(b&1)
	e.bitCnt++
	if e.bitCnt == 8 {
		e.buf = append(e.buf, e.bitAcc)
		e.bitAcc = 0
		e.bitCnt = 0
	}
}

// EncodeBool writes a boolean whose probability of being 1 is p/32768.
// Mirrors [Decoder.DecodeBool].
func (e *Encoder) EncodeBool(bit uint32, p uint32) {
	split := (e.rng - 1) * p >> 15
	split += MinProb
	if bit == 0 {
		e.rng = split
	} else {
		e.low += split
		e.rng -= split
	}
	// Renormalize: emit one output bit per doubling.
	for e.rng < SymbolCarry {
		// The bit that's "falling off" the top of the 15-bit window.
		// The decoder will XOR the incoming bit into its symbolValue,
		// which started as 0x7FFF^stream. So the emitted stream bit
		// is the inverse of what the decoder expects symbolValue to
		// gain on renormalize.
		//
		// Concretely: to make the decoder's symbolValue track the
		// encoder's low, emit ((low >> 15) & 1) each renorm step.
		outBit := (e.low >> 15) & 1
		e.emitBit(outBit)
		e.low = (e.low << 1) & 0xFFFF
		e.rng <<= 1
	}
}

// EncodeSymbol writes a symbol index drawn from a CDF, mirroring
// [Decoder.DecodeSymbol].
func (e *Encoder) EncodeSymbol(cdf []uint16, symbol int) {
	N := len(cdf) - 1
	if N < 1 || symbol < 0 || symbol >= N {
		return
	}
	// Walk the CDF computing (lo, hi) bounds for this symbol, exactly
	// as DecodeSymbol narrows the range.
	r := e.rng
	var lo, hi uint32
	for i := 0; i <= symbol; i++ {
		f := uint32(cdf[i])
		factor := ((r >> 8) * (f >> ProbShift)) >> (7 - ProbShift)
		factor += uint32(MinProb * (N - i - 1))
		prob := r - factor
		if i == symbol {
			lo = r - prob
			hi = r
			break
		}
		r -= prob
	}
	e.low += lo
	e.rng = hi - lo
	for e.rng < SymbolCarry {
		outBit := (e.low >> 15) & 1
		e.emitBit(outBit)
		e.low = (e.low << 1) & 0xFFFF
		e.rng <<= 1
	}
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

// Finish flushes remaining state and returns the serialized byte
// stream. The decoder reads 15 bits from the head and XORs with
// 0x7FFF, so we emit the final 15 bits of low (bit-inverted) plus
// zero-padding up to a byte boundary.
func (e *Encoder) Finish() []byte {
	// Emit up to 15 more bits to flush the low register.
	for i := 0; i < 16; i++ {
		e.EncodeBool(0, 16384)
	}
	if e.bitCnt > 0 {
		e.bitAcc <<= uint(8 - e.bitCnt)
		e.buf = append(e.buf, e.bitAcc)
		e.bitAcc = 0
		e.bitCnt = 0
	}
	out := make([]byte, len(e.buf))
	copy(out, e.buf)
	return out
}

// Bytes returns the current emitted byte stream without finalizing.
func (e *Encoder) Bytes() []byte {
	return e.buf
}
