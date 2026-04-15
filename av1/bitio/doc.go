// Package bitio implements the low-level bit-stream primitives defined in
// the AV1 Bitstream & Decoding Process Specification §4 (section
// "Parsing process"). Higher-level OBU parsers in goavif/av1/obu consume a
// [Reader] returned by [NewReader].
package bitio
