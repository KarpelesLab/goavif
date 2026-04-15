// Package isobmff implements reading and writing of the ISO Base Media File
// Format (ISO/IEC 14496-12) boxes used by AVIF (ISO/IEC 23008-12).
//
// The package is intentionally low-level: it exposes a generic [Box]
// representation plus concrete types for the subset of boxes needed by AVIF.
// Callers who want a structured AVIF view should use the higher-level
// [Container] type and its Parse/Serialize helpers.
package isobmff
