// Package decoder is the top-level AV1 bitstream decoder. It consumes a
// sequence header (typically from an AVIF av1C box) and a frame OBU, then
// drives the layered components in goavif/av1/{entropy,predict,transform}
// to produce a [Frame] of planar YUV samples.
//
// The decoder is in active development; see ROADMAP.md at the repository
// root for the subset of bitstream features currently implemented.
package decoder
