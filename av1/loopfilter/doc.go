// Package loopfilter implements AV1's deblocking loop filter (spec §7.14).
//
// The filter operates on edges between transform blocks. For each edge the
// decoder computes a mask + threshold set from quantization parameters and
// local pixel variation, then applies one of three filter widths (narrow,
// short, wide) to suppress blocking artifacts while preserving real edges.
//
// Only the 4-tap narrow filter is implemented so far.
package loopfilter
