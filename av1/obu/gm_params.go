package obu

import "github.com/KarpelesLab/goavif/av1/bitio"

// Global motion type codes per spec §6.8.17.
const (
	GMIdentity    = 0
	GMTranslation = 1
	GMRotZoom     = 2
	GMAffine      = 3
)

// parseGlobalMotionParams decodes global_motion_params() (spec §5.9.24) for
// inter frames. For intra-only frames the spec skips this block; callers
// should gate accordingly. Motion parameters themselves aren't stored by the
// still-image decoder so we consume them and discard.
func parseGlobalMotionParams(r *bitio.Reader, fh *FrameHeader) {
	for ref := uint(LastFrame); ref <= Altref_Frame; ref++ {
		typ := uint8(GMIdentity)
		if r.F(1) == 1 { // is_global
			if r.F(1) == 1 { // is_rot_zoom
				typ = GMRotZoom
			} else {
				if r.F(1) == 1 {
					typ = GMTranslation
				} else {
					typ = GMAffine
				}
			}
		}
		fh.GmType[ref] = typ
		switch typ {
		case GMAffine, GMRotZoom:
			readGmParam(r)
			readGmParam(r)
			if typ == GMAffine {
				readGmParam(r)
				readGmParam(r)
			}
			fallthrough
		case GMTranslation:
			readGmParam(r)
			readGmParam(r)
		}
	}
}

// readGmParam reads the signed 12-bit (or 15-bit) motion parameter. For the
// still-image decoder we don't need the actual value; the spec specifies the
// exact bit width via a helper we approximate here to keep bit alignment.
// This is a known simplification and must be corrected before inter-frame
// decoding is enabled.
func readGmParam(r *bitio.Reader) {
	r.Su(12)
}
