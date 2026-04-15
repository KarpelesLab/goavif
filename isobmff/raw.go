package isobmff

import (
	"fmt"
	"io"
)

// Box is the generic interface satisfied by every typed box and by [RawBox].
//
// Implementations are expected to be value-or-pointer receivers consistently.
// Marshal returns the box payload bytes (excluding the header). Size returns
// the payload size in bytes; implementations should return the same value
// len(Marshal()) would, but may compute it without allocating.
type Box interface {
	BoxType() FourCC
	MarshalPayload() ([]byte, error)
}

// RawBox is the fallback representation for box types this package does not
// decode into a specific Go type. Payload is the bytes after the box header.
//
// For container boxes (e.g. "meta", "iprp", "ipco") callers should use the
// typed parsers which recursively decode child boxes rather than leaving them
// as RawBox.
type RawBox struct {
	Type    FourCC
	UUID    [16]byte // only meaningful when Type == "uuid"
	Payload []byte
}

// BoxType implements [Box].
func (b *RawBox) BoxType() FourCC { return b.Type }

// MarshalPayload implements [Box].
func (b *RawBox) MarshalPayload() ([]byte, error) { return b.Payload, nil }

// readRawBox reads a single box from r, returning the header and its full
// payload in memory. The payload must fit in memory; use [ReadBoxStream] for
// large mdat payloads.
func readRawBox(r io.Reader) (Header, []byte, error) {
	h, err := readHeader(r)
	if err != nil {
		return Header{}, nil, err
	}
	if h.ExtendsToEnd {
		payload, err := io.ReadAll(r)
		if err != nil {
			return h, nil, err
		}
		h.Size = h.HeaderLen + uint64(len(payload))
		h.ExtendsToEnd = false
		return h, payload, nil
	}
	plen := h.PayloadLen()
	// Guard against pathological sizes — 1 GiB per box is more than enough
	// for AVIF still images and keeps us from OOM on malformed input.
	const maxPayload = 1 << 30
	if plen > maxPayload {
		return h, nil, fmt.Errorf("%w: %q payload %d exceeds %d", ErrInvalid, h.Type, plen, maxPayload)
	}
	payload := make([]byte, plen)
	if _, err := io.ReadFull(r, payload); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return h, nil, fmt.Errorf("%w: %q: %w", ErrTruncated, h.Type, err)
		}
		return h, nil, err
	}
	return h, payload, nil
}

// writeBox writes a single box to w using the payload returned by b.
func writeBox(w io.Writer, b Box) error {
	payload, err := b.MarshalPayload()
	if err != nil {
		return err
	}
	typ := b.BoxType()
	hl := headerLen(uint64(len(payload)), typ)
	h := Header{
		Size:      hl + uint64(len(payload)),
		Type:      typ,
		HeaderLen: hl,
	}
	if rb, ok := b.(*RawBox); ok {
		h.UUID = rb.UUID
	}
	if err := writeHeader(w, h); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}
