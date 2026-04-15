package entropy

// updateCDF adapts the CDF after decoding a symbol, per spec §9.4.
//
// cdf has N+1 entries. cdf[N] is the "count" slot that determines the
// adaptation rate:
//
//	rate = 3 + (count > 15) + (count > 31) + (N > 3)
//
// The update increments cdf[N] by 1 (saturating at 32) and nudges each
// preceding entry toward 0 (before symbol) or toward (1 << 15) (after
// symbol) by an amount of 1/2^rate of its distance to the target.
func updateCDF(cdf []uint16, N int, symbol int) {
	count := cdf[N]
	rate := uint(3)
	if count >= 16 {
		rate++
	}
	if count >= 32 {
		rate++
	}
	if N > 3 {
		rate++
	}
	// Adjust count (saturating to 32).
	if count < 32 {
		cdf[N] = count + 1
	}

	tmp := uint32(0)
	for i := 0; i < N-1; i++ {
		if i == symbol {
			tmp = 1 << 15
		}
		v := uint32(cdf[i])
		cdf[i] = uint16(v - (v-tmp)>>rate)
	}
}
