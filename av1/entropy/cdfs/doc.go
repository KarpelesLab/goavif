// Package cdfs holds the default AV1 CDF (cumulative distribution
// function) tables used by the entropy decoder at each frame's reset
// point. The tables come from AV1 spec §9.4 / libaom's
// av1/common/entropymode.c.
//
// Representation: a CDF for N symbols is stored as (N+1) uint16 entries:
//
//	cdf[0..N-2]  — Q15 values P(X > i) * 32768, monotonically decreasing
//	cdf[N-1]     — always 0
//	cdf[N]       — count slot (updates on adaptation; starts at 0)
//
// Helper macros AOM_CDF2..AOM_CDF16 in libaom serialize (p0, p1, …) into
// this form via cdf[i] = 32768 - p[i]. In Go we express the same thing
// explicitly with the MakeCDFn constructors.
//
// Not all tables from the spec are included yet; tables marked with a
// TODO in the source need transcription + verification from the AV1
// reference before the decoder can consume the corresponding symbol.
package cdfs
