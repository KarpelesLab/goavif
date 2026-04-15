// Package colorspace converts between the coded YUV planes produced by the
// AV1 decoder and the RGB values Go's image package consumes.
//
// The conversion follows the CICP (Coding-Independent Code Points) fields
// carried in the AVIF container (colr/nclx) and AV1 sequence header
// (color_primaries, transfer_characteristics, matrix_coefficients,
// color_range).
//
// Only the common 8-bit matrices are implemented today.
package colorspace
