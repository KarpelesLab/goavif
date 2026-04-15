// Package entropy implements AV1's symbol decoder (the range-coded CDF
// decoder defined in spec §9.2). It exposes a [Decoder] that consumes a
// per-tile byte buffer and produces symbols driven by CDF tables from
// goavif/av1/entropy/cdfs.
package entropy
