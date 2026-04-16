package entropy

import (
	"errors"

	"github.com/KarpelesLab/goavif/av1/bitio"
)

// EC_PROB_SHIFT and EC_MIN_PROB are the quantization parameters of the AV1
// arithmetic coder (spec §3, §9.2). CDF probabilities are stored scaled by
// 2^15 with the low EC_PROB_SHIFT bits treated as update state.
const (
	ProbShift  = 6
	MinProb    = 4
	SymbolRange0 = 1 << 15 // initial range (spec: 32768)
	SymbolCarry  = 0x8000  // renormalize threshold
)

// ErrExhausted indicates the per-tile byte buffer was consumed before
// decoding completed.
var ErrExhausted = errors.New("av1/entropy: tile bitstream exhausted")

// Decoder is a per-tile symbol decoder. Reusing a decoder across tiles is
// not safe — instantiate a fresh [Decoder] at each tile boundary by calling
// [Init].
type Decoder struct {
	br           *bitio.Reader
	symbolValue  uint32
	symbolRange  uint32
	maxBits      int64 // bits remaining budget (spec's SymbolMaxBits)
	allowUpdate  bool
}

// Init constructs a decoder over the first sz bytes of buf and primes the
// symbol decoder state per spec §9.2.1. After Init, calls to Decode* will
// consume bits from the beginning of buf.
//
// When allowCDFUpdate is true, CDFs passed to Decode are updated in-place
// after each symbol decoded, matching the AV1 default behavior when
// disable_cdf_update is false.
func (d *Decoder) Init(buf []byte, sz int, allowCDFUpdate bool) error {
	if sz > len(buf) {
		sz = len(buf)
	}
	if sz == 0 {
		return ErrExhausted
	}
	d.br = bitio.NewReader(buf[:sz])
	numBits := 15
	if sz*8 < 15 {
		numBits = sz * 8
	}
	bufVal := d.br.F(uint(numBits))
	if err := d.br.Err(); err != nil {
		return err
	}
	paddedBuf := bufVal << uint(15-numBits)
	d.symbolValue = ((1 << 15) - 1) ^ paddedBuf
	d.symbolRange = SymbolRange0
	d.maxBits = int64(sz*8 - 15)
	d.allowUpdate = allowCDFUpdate
	return nil
}

// DecodeBool decodes a single boolean with probability p (0..32768) of
// returning 1. p is in the range used by raw "boolean()" calls — the CDF
// form is preferred for most contexts.
//
// The internal formulation matches spec §9.2.5: boolean(p) is a special
// case of decode_symbol where the 2-symbol CDF has entries [p, 32768].
func (d *Decoder) DecodeBool(p uint32) uint32 {
	// split = ((range - 1) * p + 128) >> 8   // spec-equivalent, scaled to range.
	split := (d.symbolRange - 1) * p >> 15
	split += MinProb
	var bit uint32
	if d.symbolValue < split {
		d.symbolRange = split
		bit = 0
	} else {
		d.symbolRange -= split
		d.symbolValue -= split
		bit = 1
	}
	d.renormalize()
	return bit
}

// DecodeSymbol decodes one symbol from the given CDF. cdf must have length
// N+1, where N is the number of symbols; cdf[N] is the update counter slot
// used when CDF update is enabled.
//
// Returns the decoded symbol index (0..N-1).
func (d *Decoder) DecodeSymbol(cdf []uint16) int {
	N := len(cdf) - 1
	if N < 1 {
		return 0
	}
	symbol := 0
	var prob uint32
	for {
		f := uint32(cdf[symbol])
		factor := ((d.symbolRange >> 8) * (f >> ProbShift)) >> (7 - ProbShift)
		factor += uint32(MinProb * (N - symbol - 1))
		prob = d.symbolRange - factor
		if d.symbolValue >= prob {
			d.symbolValue -= prob
			d.symbolRange -= prob
			symbol++
			if symbol == N-1 {
				// Spec: when the loop reaches the last symbol, it is
				// implicitly selected without an additional range update.
				break
			}
		} else {
			d.symbolRange = prob
			break
		}
	}
	d.renormalize()
	if d.allowUpdate {
		updateCDF(cdf, N, symbol)
	}
	return symbol
}

// renormalize repeats the range rescale until the coder is back in the
// normalized [2^14, 2^15) range, per spec §9.2.7.
func (d *Decoder) renormalize() {
	for d.symbolRange < SymbolCarry {
		d.symbolRange <<= 1
		d.symbolValue = (d.symbolValue << 1) | d.br.F(1)
		d.maxBits--
	}
}

// ReadLiteral reads n uncompressed bits from the entropy-coder stream
// by decoding n raw 50/50 bools. Per spec §5.9.3 this is how
// cdef_idx, delta_qindex magnitude, and similar unadapted literals are
// carried over the range-coded payload.
//
// n must be in [0, 32]; caller must ensure that bound.
func (d *Decoder) ReadLiteral(n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		v = (v << 1) | d.DecodeBool(16384)
	}
	return v
}

// Err returns any error latched by the underlying bit reader.
func (d *Decoder) Err() error { return d.br.Err() }

// MaxBits returns the number of bits still available to consume. A negative
// value means the coder over-read into the tile's trailing bit budget.
func (d *Decoder) MaxBits() int64 { return d.maxBits }
