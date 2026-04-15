package isobmff

import "fmt"

// Ispe is the ImageSpatialExtentsProperty (§6.5.3.1). Full box, version 0.
type Ispe struct {
	FullBoxHeader
	Width  uint32
	Height uint32
}

func (*Ispe) BoxType() FourCC { return TypeIspe }

func (p *Ispe) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, p.FullBoxHeader)
	b.writeU32(p.Width)
	b.writeU32(p.Height)
	return b.bytes(), nil
}

func ParseIspe(payload []byte) (*Ispe, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	p := &Ispe{FullBoxHeader: fbh}
	p.Width = c.readU32()
	p.Height = c.readU32()
	return p, c.err
}

// Pixi is the PixelInformationProperty (§6.5.6.1). Lists the bits per
// component for each channel of the coded image.
type Pixi struct {
	FullBoxHeader
	ChannelBits []uint8
}

func (*Pixi) BoxType() FourCC { return TypePixi }

func (p *Pixi) MarshalPayload() ([]byte, error) {
	if len(p.ChannelBits) > 0xff {
		return nil, fmt.Errorf("%w: pixi %d channels > 255", ErrInvalid, len(p.ChannelBits))
	}
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, p.FullBoxHeader)
	b.writeU8(uint8(len(p.ChannelBits)))
	for _, c := range p.ChannelBits {
		b.writeU8(c)
	}
	return b.bytes(), nil
}

func ParsePixi(payload []byte) (*Pixi, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	p := &Pixi{FullBoxHeader: fbh}
	n := c.readU8()
	for i := uint8(0); i < n; i++ {
		p.ChannelBits = append(p.ChannelBits, c.readU8())
	}
	return p, c.err
}

// Pasp is the PixelAspectRatioBox (§12.1.4). Carries h_spacing:v_spacing as
// two unsigned 32-bit integers. It is *not* a FullBox.
type Pasp struct {
	HSpacing uint32
	VSpacing uint32
}

func (*Pasp) BoxType() FourCC { return TypePasp }

func (p *Pasp) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.writeU32(p.HSpacing)
	b.writeU32(p.VSpacing)
	return b.bytes(), nil
}

func ParsePasp(payload []byte) (*Pasp, error) {
	c := newCursor(payload)
	p := &Pasp{HSpacing: c.readU32(), VSpacing: c.readU32()}
	return p, c.err
}

// Irot is the ImageRotation property (§6.5.10). Rotation is counter-clockwise
// in 90-degree steps, so Angle is 0..3.
type Irot struct {
	Angle uint8
}

func (*Irot) BoxType() FourCC { return TypeIrot }

func (p *Irot) MarshalPayload() ([]byte, error) {
	return []byte{p.Angle & 0x03}, nil
}

func ParseIrot(payload []byte) (*Irot, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("%w: irot empty", ErrInvalid)
	}
	return &Irot{Angle: payload[0] & 0x03}, nil
}

// Imir is the ImageMirror property (§6.5.12). Axis 0 is vertical (mirror
// across horizontal axis), 1 is horizontal.
type Imir struct {
	Axis uint8
}

func (*Imir) BoxType() FourCC { return TypeImir }

func (p *Imir) MarshalPayload() ([]byte, error) {
	return []byte{p.Axis & 0x01}, nil
}

func ParseImir(payload []byte) (*Imir, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("%w: imir empty", ErrInvalid)
	}
	return &Imir{Axis: payload[0] & 0x01}, nil
}

// AuxC is the AuxiliaryTypeProperty (§6.5.9). For AVIF alpha, AuxType is
// "urn:mpeg:mpegB:cicp:systems:auxiliary:alpha".
type AuxC struct {
	FullBoxHeader
	AuxType    string // NUL-terminated URN
	AuxSubtype []byte // optional trailing bytes
}

func (*AuxC) BoxType() FourCC { return TypeAuxC }

func (p *AuxC) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, p.FullBoxHeader)
	b.writeCString(p.AuxType)
	b.writeBytes(p.AuxSubtype)
	return b.bytes(), nil
}

func ParseAuxC(payload []byte) (*AuxC, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	p := &AuxC{FullBoxHeader: fbh}
	p.AuxType = c.readCString()
	if c.err != nil {
		return nil, c.err
	}
	if c.remaining() > 0 {
		p.AuxSubtype = append([]byte(nil), c.readBytes(c.remaining())...)
	}
	return p, nil
}

// Clap is the CleanApertureBox (§12.1.4). All four fields are signed 32-bit
// rationals expressed as (numerator, denominator).
type Clap struct {
	CleanApertureWidthN  int32
	CleanApertureWidthD  int32
	CleanApertureHeightN int32
	CleanApertureHeightD int32
	HorizOffN            int32
	HorizOffD            int32
	VertOffN             int32
	VertOffD             int32
}

func (*Clap) BoxType() FourCC { return TypeClap }

func (p *Clap) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.writeU32(uint32(p.CleanApertureWidthN))
	b.writeU32(uint32(p.CleanApertureWidthD))
	b.writeU32(uint32(p.CleanApertureHeightN))
	b.writeU32(uint32(p.CleanApertureHeightD))
	b.writeU32(uint32(p.HorizOffN))
	b.writeU32(uint32(p.HorizOffD))
	b.writeU32(uint32(p.VertOffN))
	b.writeU32(uint32(p.VertOffD))
	return b.bytes(), nil
}

func ParseClap(payload []byte) (*Clap, error) {
	c := newCursor(payload)
	p := &Clap{
		CleanApertureWidthN:  int32(c.readU32()),
		CleanApertureWidthD:  int32(c.readU32()),
		CleanApertureHeightN: int32(c.readU32()),
		CleanApertureHeightD: int32(c.readU32()),
		HorizOffN:            int32(c.readU32()),
		HorizOffD:            int32(c.readU32()),
		VertOffN:             int32(c.readU32()),
		VertOffD:             int32(c.readU32()),
	}
	return p, c.err
}
