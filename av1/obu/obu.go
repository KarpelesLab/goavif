package obu

import (
	"errors"
	"fmt"

	"github.com/KarpelesLab/goavif/av1/bitio"
)

// Type identifies the kind of OBU carried by an [OBU]. Values match spec
// §6.2.1 (Table: OBU type codes).
type Type uint8

const (
	TypeReserved0           Type = 0
	TypeSequenceHeader      Type = 1
	TypeTemporalDelimiter   Type = 2
	TypeFrameHeader         Type = 3
	TypeTileGroup           Type = 4
	TypeMetadata            Type = 5
	TypeFrame               Type = 6
	TypeRedundantFrameHeader Type = 7
	TypeTileList            Type = 8
	TypePadding             Type = 15
)

// String returns the spec name of the OBU type.
func (t Type) String() string {
	switch t {
	case TypeSequenceHeader:
		return "OBU_SEQUENCE_HEADER"
	case TypeTemporalDelimiter:
		return "OBU_TEMPORAL_DELIMITER"
	case TypeFrameHeader:
		return "OBU_FRAME_HEADER"
	case TypeTileGroup:
		return "OBU_TILE_GROUP"
	case TypeMetadata:
		return "OBU_METADATA"
	case TypeFrame:
		return "OBU_FRAME"
	case TypeRedundantFrameHeader:
		return "OBU_REDUNDANT_FRAME_HEADER"
	case TypeTileList:
		return "OBU_TILE_LIST"
	case TypePadding:
		return "OBU_PADDING"
	}
	return fmt.Sprintf("OBU_RESERVED_%d", uint8(t))
}

// Header is the 1-or-2 byte OBU header (spec §5.3.2).
type Header struct {
	Type           Type
	ExtensionFlag  bool
	HasSizeField   bool
	TemporalID     uint8 // valid when ExtensionFlag
	SpatialID      uint8 // valid when ExtensionFlag
}

// OBU is a single parsed OBU: its header plus the payload bytes.
// Payload aliases into the input buffer and MUST NOT be mutated by callers.
type OBU struct {
	Header  Header
	Payload []byte
}

// ErrMalformed is returned when the OBU stream does not conform to spec.
var ErrMalformed = errors.New("av1/obu: malformed bitstream")

// Split parses a byte slice containing one or more concatenated OBUs whose
// obu_has_size_field bit is set. This matches the form used by AVIF (both
// inside av1C's ConfigOBUs and inside a mdat item's bitstream).
//
// For a bitstream without size fields — e.g. a low-overhead AV1 format where
// OBU sizes are implicit from an enclosing container — callers should use
// [Parse] with explicit lengths instead.
func Split(data []byte) ([]OBU, error) {
	var out []OBU
	for len(data) > 0 {
		obu, consumed, err := ParseOne(data)
		if err != nil {
			return out, err
		}
		out = append(out, obu)
		data = data[consumed:]
	}
	return out, nil
}

// ParseOne decodes a single OBU from the front of data. It requires the OBU
// to carry an explicit size field. Returns the parsed OBU and the total
// bytes consumed (header + size-leb128 + payload).
func ParseOne(data []byte) (OBU, int, error) {
	if len(data) == 0 {
		return OBU{}, 0, fmt.Errorf("%w: empty OBU buffer", ErrMalformed)
	}
	r := bitio.NewReader(data)
	h, err := parseHeader(r)
	if err != nil {
		return OBU{}, 0, err
	}
	if !h.HasSizeField {
		return OBU{}, 0, fmt.Errorf("%w: %s without obu_has_size_field", ErrMalformed, h.Type)
	}
	size, szBytes := r.Leb128()
	if err := r.Err(); err != nil {
		return OBU{}, 0, fmt.Errorf("%w: size leb128: %w", ErrMalformed, err)
	}
	headerBytes := int(r.BytePos())
	if r.BitPos()%8 != 0 {
		// Header is by definition byte-aligned.
		return OBU{}, 0, fmt.Errorf("%w: OBU header not byte-aligned", ErrMalformed)
	}
	_ = szBytes
	total := headerBytes + int(size)
	if total > len(data) {
		return OBU{}, 0, fmt.Errorf("%w: %s size %d exceeds available %d", ErrMalformed, h.Type, size, len(data)-headerBytes)
	}
	return OBU{
		Header:  h,
		Payload: data[headerBytes:total],
	}, total, nil
}

// parseHeader reads the 1-or-2 byte header from r and advances it to the
// first byte after the header.
func parseHeader(r *bitio.Reader) (Header, error) {
	if r.F(1) != 0 {
		return Header{}, fmt.Errorf("%w: obu_forbidden_bit set", ErrMalformed)
	}
	typ := Type(r.F(4))
	extFlag := r.F(1) == 1
	hasSize := r.F(1) == 1
	if r.F(1) != 0 {
		return Header{}, fmt.Errorf("%w: obu_reserved_1bit set", ErrMalformed)
	}
	h := Header{
		Type:          typ,
		ExtensionFlag: extFlag,
		HasSizeField:  hasSize,
	}
	if extFlag {
		h.TemporalID = uint8(r.F(3))
		h.SpatialID = uint8(r.F(2))
		if r.F(3) != 0 {
			return Header{}, fmt.Errorf("%w: extension_header_reserved_3bits set", ErrMalformed)
		}
	}
	if err := r.Err(); err != nil {
		return Header{}, fmt.Errorf("%w: header: %w", ErrMalformed, err)
	}
	return h, nil
}

// Parse treats the first obuLen bytes of data as a single OBU whose size is
// given externally (no obu_has_size_field). It is the low-overhead form used
// when an enclosing container already knows each OBU's length.
func Parse(data []byte, obuLen int) (OBU, error) {
	if obuLen > len(data) {
		return OBU{}, fmt.Errorf("%w: declared OBU length %d > available %d", ErrMalformed, obuLen, len(data))
	}
	r := bitio.NewReader(data[:obuLen])
	h, err := parseHeader(r)
	if err != nil {
		return OBU{}, err
	}
	headerBytes := int(r.BytePos())
	return OBU{Header: h, Payload: data[headerBytes:obuLen]}, nil
}
