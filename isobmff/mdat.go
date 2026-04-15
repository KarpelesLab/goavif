package isobmff

// Mdat is the Media Data Box (§8.1.1). It carries raw coded data whose
// meaning is determined by the sibling meta/iloc structure. For AVIF the
// mdat holds the AV1 bitstream for each item, packed back-to-back.
type Mdat struct {
	Data []byte
}

// BoxType implements [Box].
func (*Mdat) BoxType() FourCC { return TypeMdat }

// MarshalPayload implements [Box].
func (m *Mdat) MarshalPayload() ([]byte, error) { return m.Data, nil }

// ParseMdat wraps the raw payload in an [Mdat]. The returned slice aliases
// payload; callers that want independence should copy.
func ParseMdat(payload []byte) (*Mdat, error) { return &Mdat{Data: payload}, nil }
