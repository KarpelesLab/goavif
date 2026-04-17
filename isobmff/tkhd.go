package isobmff

import "encoding/binary"

// TkhdDisplaySize extracts the display width and height (in pixels)
// from the raw tkhd payload. Width and height are stored as 32.16
// fixed-point values at offsets 76 (v0) or 88 (v1) from the start
// of the payload, per ISO/IEC 14496-12 §8.3.2. Returns (0, 0) if
// the payload is malformed or too short.
func TkhdDisplaySize(payload []byte) (width, height uint32) {
	if len(payload) < 4 {
		return 0, 0
	}
	version := payload[0]
	var off int
	switch version {
	case 0:
		off = 76
	case 1:
		off = 88
	default:
		return 0, 0
	}
	if len(payload) < off+8 {
		return 0, 0
	}
	// 32.16 fixed-point — take the integer part (high 16 bits).
	w := binary.BigEndian.Uint32(payload[off : off+4])
	h := binary.BigEndian.Uint32(payload[off+4 : off+8])
	return w >> 16, h >> 16
}

// FindTkhdDisplaySize walks moov/trak/tkhd in m and returns the
// display dimensions from the first track's tkhd. Returns (0, 0) if
// not found.
func FindTkhdDisplaySize(m *Moov) (width, height uint32) {
	if m == nil {
		return 0, 0
	}
	for _, ch := range m.Children {
		trak, ok := ch.(*Trak)
		if !ok {
			continue
		}
		for _, tc := range trak.Children {
			if rb, ok := tc.(*RawBox); ok && rb.Type == TypeTkhd {
				return TkhdDisplaySize(rb.Payload)
			}
		}
	}
	return 0, 0
}
