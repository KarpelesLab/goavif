// Package transform implements AV1's forward and inverse transforms:
// DCT, ADST, FLIPADST, IDTX, and WHT, for block sizes 4, 8, 16, 32, and
// 64. The spec reference is §7.7.
//
// Only a subset is currently implemented; consult the source for coverage.
package transform
