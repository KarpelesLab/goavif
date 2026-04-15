package isobmff

import (
	"bytes"
	"io"
)

// WriteBoxes writes each box in list to w in order.
func WriteBoxes(w io.Writer, list []Box) error {
	for _, b := range list {
		if err := writeBox(w, b); err != nil {
			return err
		}
	}
	return nil
}

// EncodeChildren serializes a list of child boxes as the payload of a
// container box.
func EncodeChildren(list []Box) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteBoxes(&buf, list); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
