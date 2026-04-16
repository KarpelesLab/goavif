// Package cdef implements the Constrained Directional Enhancement Filter
// (AV1 spec §7.15).
//
// CDEF is a post-reconstruction, pre-loop-restoration filter that reduces
// ringing artifacts around edges. It operates per-8×8 block in two
// phases:
//
//  1. Direction search: pick one of 8 directions that best matches the
//     dominant gradient within the block (variance minimization after
//     subtracting a per-direction "line mean").
//
//  2. Filter: apply a primary filter along the chosen direction plus a
//     secondary filter along the perpendicular direction, using a
//     constrain() nonlinearity that limits large differences.
//
// Only the 8-bit path is implemented today.
package cdef
