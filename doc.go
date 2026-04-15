// Package goavif implements a pure-Go AVIF image codec.
//
// AVIF is an ISOBMFF-derived container (ISO/IEC 23008-12 HEIF) carrying still
// images, image sequences, or auxiliary images coded with AV1
// (ISO/IEC 23091-4). This package provides container parsing and serialization
// in [goavif/isobmff], AV1 bitstream coding in [goavif/av1], and a top-level
// API that conforms to the image.Image and image.RegisterFormat conventions.
//
// The implementation is pure Go with no cgo and no third-party dependencies in
// the core codec path.
package goavif
