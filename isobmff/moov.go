package isobmff

// Moov is the movie box container (§8.2.1). For AVIF image sequences
// it carries a single track with timing and per-sample offset info.
type Moov struct {
	Children []Box
}

func (*Moov) BoxType() FourCC { return TypeMoov }

func (m *Moov) MarshalPayload() ([]byte, error) {
	return EncodeChildren(m.Children)
}

func ParseMoov(payload []byte) (*Moov, error) {
	children, err := ReadChildren(payload)
	if err != nil {
		return nil, err
	}
	for i, ch := range children {
		rb, ok := ch.(*RawBox)
		if !ok {
			continue
		}
		switch rb.Type {
		case TypeMvhd:
			mvhd, err := ParseMvhd(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = mvhd
		case TypeTrak:
			trak, err := ParseTrak(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = trak
		}
	}
	return &Moov{Children: children}, nil
}

// Mvhd is the movie header box (§8.2.2). Only the fields relevant to
// image-sequence playback are decoded.
type Mvhd struct {
	FullBoxHeader
	CreationTime     uint64
	ModificationTime uint64
	Timescale        uint32
	Duration         uint64
	Rate             uint32 // 16.16 fixed (playback rate)
	Volume           uint16 // 8.8 fixed
	NextTrackID      uint32
}

func (*Mvhd) BoxType() FourCC { return TypeMvhd }

func ParseMvhd(payload []byte) (*Mvhd, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	m := &Mvhd{FullBoxHeader: fbh}
	if fbh.Version == 1 {
		m.CreationTime = c.readU64()
		m.ModificationTime = c.readU64()
		m.Timescale = c.readU32()
		m.Duration = c.readU64()
	} else {
		m.CreationTime = uint64(c.readU32())
		m.ModificationTime = uint64(c.readU32())
		m.Timescale = c.readU32()
		m.Duration = uint64(c.readU32())
	}
	m.Rate = c.readU32()
	m.Volume = c.readU16()
	_ = c.readU16()        // reserved
	_ = c.readBytes(8)     // reserved
	_ = c.readBytes(4 * 9) // display matrix
	_ = c.readBytes(4 * 6) // pre-defined
	m.NextTrackID = c.readU32()
	if c.err != nil {
		return nil, c.err
	}
	return m, nil
}

func (m *Mvhd) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	// Force version 0 (32-bit durations) for broad decoder compat.
	fbh := m.FullBoxHeader
	fbh.Version = 0
	b.buf = appendFullBoxHeader(b.buf, fbh)
	b.writeU32(uint32(m.CreationTime))
	b.writeU32(uint32(m.ModificationTime))
	b.writeU32(m.Timescale)
	b.writeU32(uint32(m.Duration))
	b.writeU32(m.Rate)
	b.writeU16(m.Volume)
	b.writeU16(0) // reserved
	for i := 0; i < 2; i++ {
		b.writeU32(0) // reserved[2]
	}
	// Unity display matrix (0x00010000 = 1.0 fixed).
	matrix := [9]uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000}
	for _, v := range matrix {
		b.writeU32(v)
	}
	for i := 0; i < 6; i++ {
		b.writeU32(0) // pre_defined[6]
	}
	b.writeU32(m.NextTrackID)
	return b.bytes(), nil
}

// Trak is the track box (§8.3.1). For AVIS this is the image track.
type Trak struct {
	Children []Box
}

func (*Trak) BoxType() FourCC { return TypeTrak }

func (t *Trak) MarshalPayload() ([]byte, error) {
	return EncodeChildren(t.Children)
}

func ParseTrak(payload []byte) (*Trak, error) {
	children, err := ReadChildren(payload)
	if err != nil {
		return nil, err
	}
	for i, ch := range children {
		rb, ok := ch.(*RawBox)
		if !ok {
			continue
		}
		switch rb.Type {
		case TypeMdia:
			mdia, err := ParseMdia(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = mdia
		case TypeTkhd:
			// We don't decode tkhd yet — pass through as RawBox.
		}
	}
	return &Trak{Children: children}, nil
}

// Mdia is the media box (§8.4.1). Child of trak; contains mdhd + hdlr + minf.
type Mdia struct {
	Children []Box
}

func (*Mdia) BoxType() FourCC { return TypeMdia }

func (m *Mdia) MarshalPayload() ([]byte, error) {
	return EncodeChildren(m.Children)
}

func ParseMdia(payload []byte) (*Mdia, error) {
	children, err := ReadChildren(payload)
	if err != nil {
		return nil, err
	}
	for i, ch := range children {
		rb, ok := ch.(*RawBox)
		if !ok {
			continue
		}
		if rb.Type == TypeMinf {
			minf, err := ParseMinf(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = minf
		}
	}
	return &Mdia{Children: children}, nil
}

// Minf is the media information box (§8.4.4). Contains stbl.
type Minf struct {
	Children []Box
}

func (*Minf) BoxType() FourCC { return TypeMinf }

func (m *Minf) MarshalPayload() ([]byte, error) {
	return EncodeChildren(m.Children)
}

func ParseMinf(payload []byte) (*Minf, error) {
	children, err := ReadChildren(payload)
	if err != nil {
		return nil, err
	}
	for i, ch := range children {
		rb, ok := ch.(*RawBox)
		if !ok {
			continue
		}
		if rb.Type == TypeStbl {
			stbl, err := ParseStbl(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = stbl
		}
	}
	return &Minf{Children: children}, nil
}

// Stbl is the sample table box (§8.5.1). Carries stts / stsc / stsz /
// stco / co64 which together locate each sample's bytes in the mdat.
type Stbl struct {
	Children []Box
}

func (*Stbl) BoxType() FourCC { return TypeStbl }

func (s *Stbl) MarshalPayload() ([]byte, error) {
	return EncodeChildren(s.Children)
}

func ParseStbl(payload []byte) (*Stbl, error) {
	children, err := ReadChildren(payload)
	if err != nil {
		return nil, err
	}
	for i, ch := range children {
		rb, ok := ch.(*RawBox)
		if !ok {
			continue
		}
		switch rb.Type {
		case TypeStts:
			x, err := ParseStts(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = x
		case TypeStsc:
			x, err := ParseStsc(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = x
		case TypeStsz:
			x, err := ParseStsz(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = x
		case TypeStco:
			x, err := ParseStco(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = x
		case TypeCo64:
			x, err := ParseCo64(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = x
		case TypeStss:
			x, err := ParseStss(rb.Payload)
			if err != nil {
				return nil, err
			}
			children[i] = x
		}
	}
	return &Stbl{Children: children}, nil
}
