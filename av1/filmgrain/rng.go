package filmgrain

// RNG is the 16-bit linear-feedback shift register used by AV1 film grain
// synthesis (spec §7.20.2). The generator advances state by taking the
// XOR of taps at bits 0, 1, 3 and 12 and shifting the result into bit 15.
//
// The spec seeds the RNG from grain_seed plus row/column offsets so that
// the noise is deterministic for the (frame, position) pair. Deriving the
// starting state is the caller's responsibility — this type implements
// only the step function.
type RNG struct {
	state uint16
}

// NewRNG returns an RNG primed with the given seed. A zero seed produces
// a trivial cycle, so callers should ensure the per-position seed mixes
// in non-zero bits from grain_seed.
func NewRNG(seed uint16) *RNG {
	return &RNG{state: seed}
}

// Seed resets the RNG to the given state.
func (r *RNG) Seed(seed uint16) {
	r.state = seed
}

// State returns the current LFSR value without advancing it.
func (r *RNG) State() uint16 {
	return r.state
}

// Next advances the generator one step and returns the new state. The
// returned value is a 16-bit word; individual bit widths are extracted by
// the caller (the spec uses the top 8 bits as the grain sample).
func (r *RNG) Next() uint16 {
	// Taps chosen by spec: bits 0, 1, 3, 12.
	bit := ((r.state >> 0) ^ (r.state >> 1) ^ (r.state >> 3) ^ (r.state >> 12)) & 1
	r.state = (r.state >> 1) | (bit << 15)
	return r.state
}

// Byte returns the high byte of the next state, sign-extended into an
// int. Film grain uses this as a signed 8-bit noise sample in the range
// [-128, 127].
func (r *RNG) Byte() int {
	v := r.Next()
	return int(int8(v >> 8))
}
