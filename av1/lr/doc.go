// Package lr implements AV1's Loop Restoration filter family (spec
// §7.17). Two filter kinds are defined:
//
//  1. WIENER — a 7×7 separable FIR with symmetric coefficients
//     (horizontal then vertical 1D passes, 4 unique coefficients per
//     axis). Coded via wiener_process in the frame header.
//
//  2. SGR (self-guided restoration) — a two-pass Gaussian-like filter
//     with per-unit edge-preserving weights.
//
// Per-restoration-unit signaling (use_wiener / use_sgrproj and their
// coefficient blobs) is handled by the tile decoder; this package
// provides the filter primitives.
//
// Only the Wiener filter is implemented today.
package lr
