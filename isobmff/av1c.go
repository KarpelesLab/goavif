package isobmff

import "fmt"

// Av1C is the AV1 Codec Configuration Box. The structure tracks
// "AV1 Codec ISO Media File Format Binding" §2.3.
//
// Marker is always 1 and Version is always 1 as of the spec; they are
// exposed only for round-trip fidelity. The ConfigOBUs blob contains the
// AV1 Sequence Header OBU (and optionally Metadata OBUs) concatenated
// verbatim.
type Av1C struct {
	Marker                           uint8 // 1
	Version                          uint8 // 1 (low 7 bits)
	SeqProfile                       uint8 // 0..7
	SeqLevelIdx0                     uint8 // 0..31
	SeqTier0                         uint8 // 0..1
	HighBitdepth                     uint8 // 0..1
	TwelveBit                        uint8 // 0..1
	Monochrome                       uint8 // 0..1
	ChromaSubsamplingX               uint8 // 0..1
	ChromaSubsamplingY               uint8 // 0..1
	ChromaSamplePosition             uint8 // 0..3
	InitialPresentationDelayPresent  uint8 // 0..1
	InitialPresentationDelayMinusOne uint8 // 0..15, only when InitialPresentationDelayPresent
	ConfigOBUs                       []byte
}

func (*Av1C) BoxType() FourCC { return TypeAv1C }

func (p *Av1C) MarshalPayload() ([]byte, error) {
	if p.SeqProfile > 7 {
		return nil, fmt.Errorf("%w: av1C seq_profile %d > 7", ErrInvalid, p.SeqProfile)
	}
	if p.SeqLevelIdx0 > 31 {
		return nil, fmt.Errorf("%w: av1C seq_level_idx_0 %d > 31", ErrInvalid, p.SeqLevelIdx0)
	}
	if p.ChromaSamplePosition > 3 {
		return nil, fmt.Errorf("%w: av1C chroma_sample_position %d > 3", ErrInvalid, p.ChromaSamplePosition)
	}
	marker := p.Marker
	if marker == 0 {
		marker = 1
	}
	ver := p.Version
	if ver == 0 {
		ver = 1
	}
	b := newBuilder()
	b.writeU8((marker&0x01)<<7 | (ver & 0x7f))
	b.writeU8((p.SeqProfile&0x07)<<5 | (p.SeqLevelIdx0 & 0x1f))
	byte3 := (p.SeqTier0&0x01)<<7 |
		(p.HighBitdepth&0x01)<<6 |
		(p.TwelveBit&0x01)<<5 |
		(p.Monochrome&0x01)<<4 |
		(p.ChromaSubsamplingX&0x01)<<3 |
		(p.ChromaSubsamplingY&0x01)<<2 |
		(p.ChromaSamplePosition & 0x03)
	b.writeU8(byte3)
	byte4 := (p.InitialPresentationDelayPresent & 0x01) << 4
	if p.InitialPresentationDelayPresent != 0 {
		byte4 |= p.InitialPresentationDelayMinusOne & 0x0f
	}
	b.writeU8(byte4)
	b.writeBytes(p.ConfigOBUs)
	return b.bytes(), nil
}

func ParseAv1C(payload []byte) (*Av1C, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("%w: av1C payload %d < 4 bytes", ErrInvalid, len(payload))
	}
	p := &Av1C{}
	p.Marker = payload[0] >> 7
	p.Version = payload[0] & 0x7f
	p.SeqProfile = payload[1] >> 5
	p.SeqLevelIdx0 = payload[1] & 0x1f
	p.SeqTier0 = (payload[2] >> 7) & 0x01
	p.HighBitdepth = (payload[2] >> 6) & 0x01
	p.TwelveBit = (payload[2] >> 5) & 0x01
	p.Monochrome = (payload[2] >> 4) & 0x01
	p.ChromaSubsamplingX = (payload[2] >> 3) & 0x01
	p.ChromaSubsamplingY = (payload[2] >> 2) & 0x01
	p.ChromaSamplePosition = payload[2] & 0x03
	p.InitialPresentationDelayPresent = (payload[3] >> 4) & 0x01
	if p.InitialPresentationDelayPresent != 0 {
		p.InitialPresentationDelayMinusOne = payload[3] & 0x0f
	}
	if len(payload) > 4 {
		p.ConfigOBUs = append([]byte(nil), payload[4:]...)
	}
	return p, nil
}
