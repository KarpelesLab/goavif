package isobmff

import (
	"bytes"
	"fmt"
	"io"
)

// ReadBoxes reads successive boxes from r until EOF and returns them as a
// flat slice in file order. Container boxes are returned as [RawBox]; call
// [ReadChildren] to recursively descend.
func ReadBoxes(r io.Reader) ([]Box, error) {
	var out []Box
	for {
		h, payload, err := readRawBox(r)
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		rb := &RawBox{Type: h.Type, UUID: h.UUID, Payload: payload}
		out = append(out, rb)
	}
}

// ReadChildren reads boxes from payload, treating it as the contents of a
// container box. It returns the children as a flat slice.
func ReadChildren(payload []byte) ([]Box, error) {
	return ReadBoxes(bytes.NewReader(payload))
}

// FindBox returns the first box in list whose type equals t, or nil.
func FindBox(list []Box, t FourCC) Box {
	for _, b := range list {
		if b.BoxType() == t {
			return b
		}
	}
	return nil
}

// FindAll returns every box in list whose type equals t in order.
func FindAll(list []Box, t FourCC) []Box {
	var out []Box
	for _, b := range list {
		if b.BoxType() == t {
			out = append(out, b)
		}
	}
	return out
}

// RawPayloadOf returns the raw payload bytes of b. It works for [RawBox] and
// re-marshals typed boxes for anything else.
func RawPayloadOf(b Box) ([]byte, error) {
	if rb, ok := b.(*RawBox); ok {
		return rb.Payload, nil
	}
	return b.MarshalPayload()
}

// ExpectType returns an error if b's type is not t.
func ExpectType(b Box, t FourCC) error {
	if b.BoxType() != t {
		return fmt.Errorf("%w: expected %q, got %q", ErrInvalid, t, b.BoxType())
	}
	return nil
}
