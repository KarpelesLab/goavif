package isobmff

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// FourCC is a four-character code identifying a box type.
type FourCC [4]byte

// String returns a human-readable form of the FourCC. Non-printable bytes
// are rendered as '.'.
func (f FourCC) String() string {
	var b [4]byte
	for i, c := range f {
		if c >= 0x20 && c < 0x7f {
			b[i] = c
		} else {
			b[i] = '.'
		}
	}
	return string(b[:])
}

// MarshalText implements [encoding.TextMarshaler] for friendly JSON/text output.
func (f FourCC) MarshalText() ([]byte, error) { return []byte(f.String()), nil }

// Equal reports whether f equals s. s must be exactly 4 bytes.
func (f FourCC) Equal(s string) bool {
	if len(s) != 4 {
		return false
	}
	return string(f[:]) == s
}

// FourCCOf converts a four-character ASCII string to a FourCC.
// It panics if s is not exactly 4 bytes.
func FourCCOf(s string) FourCC {
	if len(s) != 4 {
		panic(fmt.Sprintf("isobmff: FourCCOf(%q): need exactly 4 bytes", s))
	}
	var f FourCC
	copy(f[:], s)
	return f
}

// Known box types used throughout the AVIF specification. Only the subset we
// actually read or write is listed here; unknown types pass through as [RawBox].
var (
	TypeFtyp = FourCCOf("ftyp")
	TypeMeta = FourCCOf("meta")
	TypeHdlr = FourCCOf("hdlr")
	TypePitm = FourCCOf("pitm")
	TypeIloc = FourCCOf("iloc")
	TypeIinf = FourCCOf("iinf")
	TypeInfe = FourCCOf("infe")
	TypeIref = FourCCOf("iref")
	TypeIprp = FourCCOf("iprp")
	TypeIpco = FourCCOf("ipco")
	TypeIpma = FourCCOf("ipma")
	TypeMdat = FourCCOf("mdat")
	TypeFree = FourCCOf("free")
	TypeSkip = FourCCOf("skip")
	TypeUUID = FourCCOf("uuid")

	// Property boxes carried under ipco.
	TypeIspe = FourCCOf("ispe")
	TypePixi = FourCCOf("pixi")
	TypeAv1C = FourCCOf("av1C")
	TypeColr = FourCCOf("colr")
	TypePasp = FourCCOf("pasp")
	TypeIrot = FourCCOf("irot")
	TypeImir = FourCCOf("imir")
	TypeAuxC = FourCCOf("auxC")
	TypeClap = FourCCOf("clap")

	// Reference box types under iref.
	TypeAuxl = FourCCOf("auxl")
	TypeThmb = FourCCOf("thmb")
	TypeCdsc = FourCCOf("cdsc")
	TypeDimg = FourCCOf("dimg")
)

// ErrTruncated indicates that a box ended before its declared size.
var ErrTruncated = errors.New("isobmff: truncated box")

// ErrInvalid indicates an otherwise malformed box.
var ErrInvalid = errors.New("isobmff: invalid box")

// Header is the common leading data of every ISOBMFF box.
//
// The wire layout is:
//
//	unsigned int(32) size;
//	unsigned int(32) type;
//	if (size == 1) unsigned int(64) largesize;
//	if (size == 0) // box extends to end of stream
//	if (type == "uuid") unsigned int(8)[16] usertype;
//
// Size reports the total number of bytes the box occupies on the wire,
// including the header itself. HeaderLen is the number of bytes consumed by
// the header, so the payload length is Size - HeaderLen.
//
// A Size of zero means the box runs to end of stream; the caller is expected
// to substitute the remaining stream length when writing.
type Header struct {
	Size      uint64
	Type      FourCC
	UUID      [16]byte // only meaningful when Type == "uuid"
	HeaderLen uint64
	// ExtendsToEnd is true if the on-wire size field was zero.
	ExtendsToEnd bool
}

// PayloadLen returns the length of the box payload (Size - HeaderLen), or
// zero if the box is flagged as extending to end of stream.
func (h Header) PayloadLen() uint64 {
	if h.ExtendsToEnd {
		return 0
	}
	if h.Size < h.HeaderLen {
		return 0
	}
	return h.Size - h.HeaderLen
}

// readHeader reads a box header from r. It does not read any payload.
// total is the number of bytes remaining in the enclosing stream/container
// and is used only when size == 0 to compute ExtendsToEnd's effective size.
func readHeader(r io.Reader) (Header, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return Header{}, err
	}
	h := Header{
		Size:      uint64(binary.BigEndian.Uint32(buf[0:4])),
		HeaderLen: 8,
	}
	copy(h.Type[:], buf[4:8])

	switch h.Size {
	case 1:
		var ls [8]byte
		if _, err := io.ReadFull(r, ls[:]); err != nil {
			return Header{}, err
		}
		h.Size = binary.BigEndian.Uint64(ls[:])
		h.HeaderLen = 16
	case 0:
		h.ExtendsToEnd = true
	}

	if h.Type == TypeUUID {
		if _, err := io.ReadFull(r, h.UUID[:]); err != nil {
			return Header{}, err
		}
		h.HeaderLen += 16
	}

	if !h.ExtendsToEnd && h.Size < h.HeaderLen {
		return Header{}, fmt.Errorf("%w: %q size %d < header %d", ErrInvalid, h.Type, h.Size, h.HeaderLen)
	}
	return h, nil
}

// writeHeader writes the box header for a box of the given total size to w.
// size must include the header bytes. A size of zero writes the "extends to
// EOF" form and is only valid for the final top-level box.
func writeHeader(w io.Writer, h Header) error {
	useLarge := h.Size >= 1<<32
	var buf []byte
	if useLarge {
		buf = make([]byte, 0, 16)
		buf = binary.BigEndian.AppendUint32(buf, 1)
		buf = append(buf, h.Type[:]...)
		buf = binary.BigEndian.AppendUint64(buf, h.Size)
	} else {
		buf = make([]byte, 0, 8)
		if h.ExtendsToEnd {
			buf = binary.BigEndian.AppendUint32(buf, 0)
		} else {
			buf = binary.BigEndian.AppendUint32(buf, uint32(h.Size))
		}
		buf = append(buf, h.Type[:]...)
	}
	if _, err := w.Write(buf); err != nil {
		return err
	}
	if h.Type == TypeUUID {
		if _, err := w.Write(h.UUID[:]); err != nil {
			return err
		}
	}
	return nil
}

// headerLen returns the on-wire size of the header for a box of payload
// length payloadLen and the given type/uuid flag.
func headerLen(payloadLen uint64, typ FourCC) uint64 {
	hdr := uint64(8)
	if typ == TypeUUID {
		hdr += 16
	}
	if payloadLen+hdr >= 1<<32 {
		hdr += 8 // largesize
	}
	return hdr
}

// FullBoxHeader is the extra version/flags word prefixed to every "FullBox".
type FullBoxHeader struct {
	Version uint8
	Flags   uint32 // only low 24 bits used
}

// readFullBoxHeader reads the 4-byte version+flags prefix from p and returns
// the header plus the remaining payload slice.
func readFullBoxHeader(p []byte) (FullBoxHeader, []byte, error) {
	if len(p) < 4 {
		return FullBoxHeader{}, nil, fmt.Errorf("%w: full box header needs 4 bytes", ErrInvalid)
	}
	return FullBoxHeader{
		Version: p[0],
		Flags:   uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3]),
	}, p[4:], nil
}

// appendFullBoxHeader appends the 4-byte version+flags prefix to buf.
func appendFullBoxHeader(buf []byte, fbh FullBoxHeader) []byte {
	return append(buf, fbh.Version, byte(fbh.Flags>>16), byte(fbh.Flags>>8), byte(fbh.Flags))
}
