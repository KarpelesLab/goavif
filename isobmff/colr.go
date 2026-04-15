package isobmff

import "fmt"

// ColrType selects the representation carried inside a [Colr] box.
type ColrType FourCC

var (
	// ColrTypeNCLX is the CICP-style small form used by most AVIFs.
	ColrTypeNCLX = FourCCOf("nclx")
	// ColrTypeRICC carries a restricted ICC profile.
	ColrTypeRICC = FourCCOf("rICC")
	// ColrTypeProf carries an unrestricted ICC profile.
	ColrTypeProf = FourCCOf("prof")
)

// Colr is the ColourInformationBox (§12.1.5). Only one of NCLX or ICC is set
// depending on Type.
type Colr struct {
	Type FourCC

	// NCLX fields, valid when Type == "nclx".
	ColourPrimaries         uint16
	TransferCharacteristics uint16
	MatrixCoefficients      uint16
	FullRange               bool

	// ICC bytes, valid when Type == "rICC" or "prof".
	ICC []byte
}

func (*Colr) BoxType() FourCC { return TypeColr }

func (p *Colr) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.writeBytes(p.Type[:])
	switch p.Type {
	case ColrTypeNCLX:
		b.writeU16(p.ColourPrimaries)
		b.writeU16(p.TransferCharacteristics)
		b.writeU16(p.MatrixCoefficients)
		if p.FullRange {
			b.writeU8(0x80)
		} else {
			b.writeU8(0x00)
		}
	case ColrTypeRICC, ColrTypeProf:
		b.writeBytes(p.ICC)
	default:
		return nil, fmt.Errorf("%w: colr unknown colour_type %q", ErrInvalid, p.Type)
	}
	return b.bytes(), nil
}

func ParseColr(payload []byte) (*Colr, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("%w: colr payload %d < 4", ErrInvalid, len(payload))
	}
	p := &Colr{}
	copy(p.Type[:], payload[:4])
	rest := payload[4:]
	switch p.Type {
	case ColrTypeNCLX:
		if len(rest) < 7 {
			return nil, fmt.Errorf("%w: nclx colr payload %d < 7", ErrInvalid, len(rest))
		}
		c := newCursor(rest)
		p.ColourPrimaries = c.readU16()
		p.TransferCharacteristics = c.readU16()
		p.MatrixCoefficients = c.readU16()
		p.FullRange = (c.readU8() & 0x80) != 0
	case ColrTypeRICC, ColrTypeProf:
		p.ICC = append([]byte(nil), rest...)
	default:
		return nil, fmt.Errorf("%w: colr unknown colour_type %q", ErrInvalid, p.Type)
	}
	return p, nil
}
