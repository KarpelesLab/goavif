package isobmff

import "fmt"

// Ftyp is the File Type Box (ISO/IEC 14496-12 §4.3).
//
// For AVIF files MajorBrand is typically "avif" (still image) or "avis"
// (image sequence), with CompatibleBrands including at minimum "avif" and
// "mif1" (still) or "avis"/"msf1" (sequence).
type Ftyp struct {
	MajorBrand       FourCC
	MinorVersion     uint32
	CompatibleBrands []FourCC
}

// BoxType implements [Box].
func (*Ftyp) BoxType() FourCC { return TypeFtyp }

// MarshalPayload implements [Box].
func (f *Ftyp) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.writeBytes(f.MajorBrand[:])
	b.writeU32(f.MinorVersion)
	for _, cb := range f.CompatibleBrands {
		b.writeBytes(cb[:])
	}
	return b.bytes(), nil
}

// ParseFtyp decodes an ftyp payload.
func ParseFtyp(payload []byte) (*Ftyp, error) {
	if len(payload) < 8 || len(payload)%4 != 0 {
		return nil, fmt.Errorf("%w: ftyp length %d", ErrInvalid, len(payload))
	}
	f := &Ftyp{}
	copy(f.MajorBrand[:], payload[:4])
	f.MinorVersion = uint32(payload[4])<<24 | uint32(payload[5])<<16 | uint32(payload[6])<<8 | uint32(payload[7])
	for i := 8; i+4 <= len(payload); i += 4 {
		var c FourCC
		copy(c[:], payload[i:i+4])
		f.CompatibleBrands = append(f.CompatibleBrands, c)
	}
	return f, nil
}

// HasBrand reports whether brand appears as MajorBrand or in CompatibleBrands.
func (f *Ftyp) HasBrand(brand string) bool {
	if len(brand) != 4 {
		return false
	}
	if f.MajorBrand.Equal(brand) {
		return true
	}
	for _, cb := range f.CompatibleBrands {
		if cb.Equal(brand) {
			return true
		}
	}
	return false
}
