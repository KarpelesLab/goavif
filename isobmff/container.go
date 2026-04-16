package isobmff

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Container is a parsed AVIF/HEIF-style file. It aggregates the top-level
// boxes this package understands; anything unrecognized is preserved in
// Extras so round-tripping is lossless.
type Container struct {
	Ftyp *Ftyp
	Meta *Meta
	// Moov is populated for AVIF image sequences (ftyp brand "avis")
	// and carries the per-frame timing and byte-offset tables. Nil
	// for still images.
	Moov *Moov
	Mdat *Mdat
	// MdatOffset is the absolute file offset of the mdat payload (first byte
	// after the mdat header). It is set by [ParseContainer] and used by
	// [Container.ItemData] to resolve iloc extents of construction_method 0.
	MdatOffset uint64
	// Extras are top-level boxes other than ftyp/meta/moov/mdat, preserved
	// in the order seen in the source file.
	Extras []Box
}

// ParseContainer parses data as an AVIF/HEIF file. On success the returned
// Container's Meta has its children promoted to typed boxes (Hdlr, Pitm,
// Iloc, Iinf, Iprp, Iref), and MdatOffset is populated.
func ParseContainer(data []byte) (*Container, error) {
	r := bytes.NewReader(data)
	ct := &Container{}
	off := uint64(0)
	for {
		hdr, err := readHeader(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		plen := int64(hdr.PayloadLen())
		if hdr.ExtendsToEnd {
			plen = int64(r.Len())
		}
		payload := make([]byte, plen)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, fmt.Errorf("%w: %q payload: %w", ErrTruncated, hdr.Type, err)
		}

		switch hdr.Type {
		case TypeFtyp:
			ft, err := ParseFtyp(payload)
			if err != nil {
				return nil, err
			}
			if ct.Ftyp != nil {
				return nil, fmt.Errorf("%w: multiple ftyp boxes", ErrInvalid)
			}
			ct.Ftyp = ft
		case TypeMeta:
			mt, err := ParseMeta(payload)
			if err != nil {
				return nil, err
			}
			if err := promoteMetaChildren(mt); err != nil {
				return nil, err
			}
			ct.Meta = mt
		case TypeMoov:
			mv, err := ParseMoov(payload)
			if err != nil {
				return nil, err
			}
			ct.Moov = mv
		case TypeMdat:
			md, err := ParseMdat(payload)
			if err != nil {
				return nil, err
			}
			ct.Mdat = md
			ct.MdatOffset = off + hdr.HeaderLen
		default:
			ct.Extras = append(ct.Extras, &RawBox{
				Type:    hdr.Type,
				UUID:    hdr.UUID,
				Payload: payload,
			})
		}

		off += hdr.HeaderLen + uint64(len(payload))
	}
	if ct.Ftyp == nil {
		return nil, fmt.Errorf("%w: no ftyp box", ErrInvalid)
	}
	return ct, nil
}

// promoteMetaChildren walks a meta box and replaces recognized RawBox
// children with their typed equivalents. Unknown boxes are left in place.
func promoteMetaChildren(m *Meta) error {
	for i, ch := range m.Children {
		rb, ok := ch.(*RawBox)
		if !ok {
			continue
		}
		var replaced Box
		var err error
		switch rb.Type {
		case TypeHdlr:
			replaced, err = ParseHdlr(rb.Payload)
		case TypePitm:
			replaced, err = ParsePitm(rb.Payload)
		case TypeIloc:
			replaced, err = ParseIloc(rb.Payload)
		case TypeIinf:
			replaced, err = ParseIinf(rb.Payload)
		case TypeIprp:
			replaced, err = ParseIprpPromoted(rb.Payload)
		case TypeIref:
			replaced, err = ParseIref(rb.Payload)
		}
		if err != nil {
			return err
		}
		if replaced != nil {
			m.Children[i] = replaced
		}
	}
	return nil
}

// ParseIprpPromoted is like [ParseIprp] but also promotes every property
// inside ipco to its typed form where known. Unknown properties stay as
// [RawBox] so the original bytes survive round-trips.
func ParseIprpPromoted(payload []byte) (*Iprp, error) {
	p, err := ParseIprp(payload)
	if err != nil {
		return nil, err
	}
	for i, prop := range p.Ipco.Properties {
		rb, ok := prop.(*RawBox)
		if !ok {
			continue
		}
		var promoted Box
		var err error
		switch rb.Type {
		case TypeIspe:
			promoted, err = ParseIspe(rb.Payload)
		case TypePixi:
			promoted, err = ParsePixi(rb.Payload)
		case TypeAv1C:
			promoted, err = ParseAv1C(rb.Payload)
		case TypeColr:
			promoted, err = ParseColr(rb.Payload)
		case TypePasp:
			promoted, err = ParsePasp(rb.Payload)
		case TypeIrot:
			promoted, err = ParseIrot(rb.Payload)
		case TypeImir:
			promoted, err = ParseImir(rb.Payload)
		case TypeAuxC:
			promoted, err = ParseAuxC(rb.Payload)
		case TypeClap:
			promoted, err = ParseClap(rb.Payload)
		}
		if err != nil {
			return nil, err
		}
		if promoted != nil {
			p.Ipco.Properties[i] = promoted
		}
	}
	return p, nil
}

// findIloc returns the Iloc box from a parsed Meta, or nil.
func (ct *Container) findIloc() *Iloc {
	if ct.Meta == nil {
		return nil
	}
	for _, ch := range ct.Meta.Children {
		if il, ok := ch.(*Iloc); ok {
			return il
		}
	}
	return nil
}

// findIinf returns the Iinf box from a parsed Meta, or nil.
func (ct *Container) findIinf() *Iinf {
	if ct.Meta == nil {
		return nil
	}
	for _, ch := range ct.Meta.Children {
		if ii, ok := ch.(*Iinf); ok {
			return ii
		}
	}
	return nil
}

// findIprp returns the Iprp box from a parsed Meta, or nil.
func (ct *Container) findIprp() *Iprp {
	if ct.Meta == nil {
		return nil
	}
	for _, ch := range ct.Meta.Children {
		if ip, ok := ch.(*Iprp); ok {
			return ip
		}
	}
	return nil
}

// PrimaryItemID returns the primary item id declared by pitm, or 0 if absent.
func (ct *Container) PrimaryItemID() uint32 {
	if ct.Meta == nil {
		return 0
	}
	for _, ch := range ct.Meta.Children {
		if p, ok := ch.(*Pitm); ok {
			return p.ItemID
		}
	}
	return 0
}

// ItemData returns the bytes for the given item id by resolving iloc extents
// against the source file. Only construction_method 0 (file offset into
// mdat) is supported currently; other methods return [ErrUnsupportedConstruction].
//
// The returned slice is a fresh copy safe to retain.
func (ct *Container) ItemData(itemID uint32) ([]byte, error) {
	iloc := ct.findIloc()
	if iloc == nil {
		return nil, fmt.Errorf("%w: no iloc", ErrInvalid)
	}
	var item *IlocItem
	for i := range iloc.Items {
		if iloc.Items[i].ItemID == itemID {
			item = &iloc.Items[i]
			break
		}
	}
	if item == nil {
		return nil, fmt.Errorf("%w: item %d not in iloc", ErrInvalid, itemID)
	}
	if item.ConstructionMethod != ConstructionFileOffset {
		return nil, fmt.Errorf("%w: item %d uses construction_method %d", ErrUnsupportedConstruction, itemID, item.ConstructionMethod)
	}
	if ct.Mdat == nil {
		return nil, fmt.Errorf("%w: item %d references mdat but none present", ErrInvalid, itemID)
	}

	var total uint64
	for _, ex := range item.Extents {
		total += ex.Length
	}
	out := make([]byte, 0, total)
	for _, ex := range item.Extents {
		fileOff := item.BaseOffset + ex.Offset
		if fileOff < ct.MdatOffset {
			return nil, fmt.Errorf("%w: item %d extent offset %d < mdat offset %d", ErrInvalid, itemID, fileOff, ct.MdatOffset)
		}
		rel := fileOff - ct.MdatOffset
		end := rel + ex.Length
		if end > uint64(len(ct.Mdat.Data)) {
			return nil, fmt.Errorf("%w: item %d extent runs past mdat (%d..%d of %d)", ErrInvalid, itemID, rel, end, len(ct.Mdat.Data))
		}
		out = append(out, ct.Mdat.Data[rel:end]...)
	}
	return out, nil
}

// ErrUnsupportedConstruction is returned when ItemData encounters a
// construction method this package does not implement (idat or item-relative).
var ErrUnsupportedConstruction = fmt.Errorf("%w: unsupported construction_method", ErrInvalid)

// WriteTo serializes the container to w. Boxes are written in this order:
// ftyp, extras (preserved order), meta, mdat. Iloc offsets inside meta are
// patched after layout so they resolve correctly against the written mdat.
func (ct *Container) WriteTo(w io.Writer) (int64, error) {
	if ct.Ftyp == nil {
		return 0, fmt.Errorf("%w: container missing ftyp", ErrInvalid)
	}
	// First pass: marshal everything to compute byte offsets.
	var ftypBuf, metaBuf, mdatBuf bytes.Buffer
	if err := writeBox(&ftypBuf, ct.Ftyp); err != nil {
		return 0, err
	}
	var extrasBuf bytes.Buffer
	for _, e := range ct.Extras {
		if err := writeBox(&extrasBuf, e); err != nil {
			return 0, err
		}
	}
	if ct.Meta != nil {
		if err := writeBox(&metaBuf, ct.Meta); err != nil {
			return 0, err
		}
	}
	if ct.Mdat != nil {
		if err := writeBox(&mdatBuf, ct.Mdat); err != nil {
			return 0, err
		}
	}

	// Compute the mdat payload offset in the output.
	headerLenOfMdat := headerLen(uint64(len(ct.Mdat.Data)), TypeMdat)
	mdatHeaderOff := uint64(ftypBuf.Len()) + uint64(extrasBuf.Len()) + uint64(metaBuf.Len())
	mdatPayloadOff := mdatHeaderOff + headerLenOfMdat

	// Patch iloc offsets in the serialized meta if they were placeholders
	// referencing ct.MdatOffset. We re-marshal meta after updating offsets.
	if ct.Mdat != nil && ct.Meta != nil {
		if err := patchIlocOffsetsInMeta(ct.Meta, mdatPayloadOff); err != nil {
			return 0, err
		}
		metaBuf.Reset()
		if err := writeBox(&metaBuf, ct.Meta); err != nil {
			return 0, err
		}
		// Size must match the pre-compute; an iloc width increase would
		// change meta size and shift mdat. AutoSize should have been called
		// before WriteTo if offsets need to fit; bail if not.
		newMdatHeaderOff := uint64(ftypBuf.Len()) + uint64(extrasBuf.Len()) + uint64(metaBuf.Len())
		if newMdatHeaderOff != mdatHeaderOff {
			return 0, fmt.Errorf("%w: iloc patch changed meta size (widths insufficient?)", ErrInvalid)
		}
	}
	// Reflect the final mdat offset on the container for callers that write
	// and then re-read their own output.
	ct.MdatOffset = mdatPayloadOff

	var n int64
	write := func(buf []byte) error {
		if len(buf) == 0 {
			return nil
		}
		k, err := w.Write(buf)
		n += int64(k)
		return err
	}
	if err := write(ftypBuf.Bytes()); err != nil {
		return n, err
	}
	if err := write(extrasBuf.Bytes()); err != nil {
		return n, err
	}
	if err := write(metaBuf.Bytes()); err != nil {
		return n, err
	}
	if err := write(mdatBuf.Bytes()); err != nil {
		return n, err
	}
	return n, nil
}

// patchIlocOffsetsInMeta rewrites the Iloc inside m so every item's effective
// absolute file offset (BaseOffset + extent.Offset) resolves to a position
// inside the mdat that starts at mdatPayloadOff.
//
// The convention on the input: item.BaseOffset is zero and extent.Offset is
// the byte offset inside the mdat payload (i.e. relative to mdatPayloadOff).
// We convert that to an absolute file offset by adding mdatPayloadOff to
// each extent.Offset while leaving BaseOffset = 0. This keeps iloc semantics
// simple and predictable.
func patchIlocOffsetsInMeta(m *Meta, mdatPayloadOff uint64) error {
	for _, ch := range m.Children {
		il, ok := ch.(*Iloc)
		if !ok {
			continue
		}
		for i := range il.Items {
			for j := range il.Items[i].Extents {
				// Treat current Offset as mdat-relative.
				il.Items[i].Extents[j].Offset += mdatPayloadOff
			}
		}
		// Intentionally no AutoSize here: changing widths would shift the
		// enclosing meta box and invalidate the mdat offset we just wrote
		// into the extents. Callers must have chosen widths large enough
		// in advance (see BuildStillImage).
	}
	return nil
}

// Encode serializes the container to a newly allocated byte slice.
func (ct *Container) Encode() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := ct.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// prepareIlocForWrite undoes patchIlocOffsetsInMeta so a container can be
// re-serialized multiple times. Applied automatically by subsequent calls.
//
// This helper is exported for callers that want to mutate a parsed container
// and write it back; they should call it before the second WriteTo.
func (ct *Container) ResetIlocOffsets() {
	if ct.Meta == nil || ct.MdatOffset == 0 {
		return
	}
	for _, ch := range ct.Meta.Children {
		il, ok := ch.(*Iloc)
		if !ok {
			continue
		}
		for i := range il.Items {
			for j := range il.Items[i].Extents {
				ex := &il.Items[i].Extents[j]
				if ex.Offset >= ct.MdatOffset {
					ex.Offset -= ct.MdatOffset
				}
			}
		}
	}
}

// compile-time check that binary is imported.
var _ = binary.BigEndian
