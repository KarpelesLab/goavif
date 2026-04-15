// Package predict implements AV1 intra and inter prediction (spec §7.10–7.11).
//
// Only DC intra prediction is currently implemented. The remaining modes
// (directional, smooth variants, Paeth, CFL, recursive filter intra, and
// all inter prediction) will land in subsequent phases.
package predict
