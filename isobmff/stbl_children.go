package isobmff

import "fmt"

// Stts is the time-to-sample box (§8.6.1.2). It carries a compact
// list of (count, delta) pairs from which per-sample presentation
// times are reconstructed.
type Stts struct {
	FullBoxHeader
	Entries []SttsEntry
}

// SttsEntry is one (count, delta) pair.
type SttsEntry struct {
	Count uint32
	Delta uint32
}

func (*Stts) BoxType() FourCC { return TypeStts }

func (s *Stts) MarshalPayload() ([]byte, error) {
	return nil, fmt.Errorf("%w: Stts.MarshalPayload not implemented", ErrInvalid)
}

func ParseStts(payload []byte) (*Stts, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	n := c.readU32()
	s := &Stts{FullBoxHeader: fbh, Entries: make([]SttsEntry, 0, n)}
	for i := uint32(0); i < n; i++ {
		s.Entries = append(s.Entries, SttsEntry{
			Count: c.readU32(),
			Delta: c.readU32(),
		})
	}
	if c.err != nil {
		return nil, c.err
	}
	return s, nil
}

// SampleDeltas returns a slice of per-sample durations derived from
// the stts entries. Length equals the total sample count.
func (s *Stts) SampleDeltas() []uint32 {
	total := uint32(0)
	for _, e := range s.Entries {
		total += e.Count
	}
	out := make([]uint32, 0, total)
	for _, e := range s.Entries {
		for i := uint32(0); i < e.Count; i++ {
			out = append(out, e.Delta)
		}
	}
	return out
}

// Stsc is the sample-to-chunk box (§8.7.4). Each entry describes a run
// of chunks that share the same samples-per-chunk and description.
type Stsc struct {
	FullBoxHeader
	Entries []StscEntry
}

type StscEntry struct {
	FirstChunk      uint32
	SamplesPerChunk uint32
	DescriptionIdx  uint32
}

func (*Stsc) BoxType() FourCC { return TypeStsc }

func (s *Stsc) MarshalPayload() ([]byte, error) {
	return nil, fmt.Errorf("%w: Stsc.MarshalPayload not implemented", ErrInvalid)
}

func ParseStsc(payload []byte) (*Stsc, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	n := c.readU32()
	s := &Stsc{FullBoxHeader: fbh, Entries: make([]StscEntry, 0, n)}
	for i := uint32(0); i < n; i++ {
		s.Entries = append(s.Entries, StscEntry{
			FirstChunk:      c.readU32(),
			SamplesPerChunk: c.readU32(),
			DescriptionIdx:  c.readU32(),
		})
	}
	if c.err != nil {
		return nil, c.err
	}
	return s, nil
}

// Stsz is the sample-size box (§8.7.3.2). Either all samples share a
// single size (SampleSize != 0) or individual sizes are listed.
type Stsz struct {
	FullBoxHeader
	SampleSize  uint32
	SampleCount uint32
	Sizes       []uint32 // populated when SampleSize == 0
}

func (*Stsz) BoxType() FourCC { return TypeStsz }

func (s *Stsz) MarshalPayload() ([]byte, error) {
	return nil, fmt.Errorf("%w: Stsz.MarshalPayload not implemented", ErrInvalid)
}

func ParseStsz(payload []byte) (*Stsz, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	s := &Stsz{FullBoxHeader: fbh}
	s.SampleSize = c.readU32()
	s.SampleCount = c.readU32()
	if s.SampleSize == 0 {
		s.Sizes = make([]uint32, 0, s.SampleCount)
		for i := uint32(0); i < s.SampleCount; i++ {
			s.Sizes = append(s.Sizes, c.readU32())
		}
	}
	if c.err != nil {
		return nil, c.err
	}
	return s, nil
}

// SizeOf returns the byte size of the 1-based sample index.
func (s *Stsz) SizeOf(sampleIndex uint32) uint32 {
	if s.SampleSize != 0 {
		return s.SampleSize
	}
	if sampleIndex < 1 || int(sampleIndex) > len(s.Sizes) {
		return 0
	}
	return s.Sizes[sampleIndex-1]
}

// Stco is the 32-bit chunk offset box (§8.7.5). Offsets are absolute
// byte positions into the source stream.
type Stco struct {
	FullBoxHeader
	Offsets []uint32
}

func (*Stco) BoxType() FourCC { return TypeStco }

func (s *Stco) MarshalPayload() ([]byte, error) {
	return nil, fmt.Errorf("%w: Stco.MarshalPayload not implemented", ErrInvalid)
}

func ParseStco(payload []byte) (*Stco, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	n := c.readU32()
	s := &Stco{FullBoxHeader: fbh, Offsets: make([]uint32, 0, n)}
	for i := uint32(0); i < n; i++ {
		s.Offsets = append(s.Offsets, c.readU32())
	}
	if c.err != nil {
		return nil, c.err
	}
	return s, nil
}

// Stss is the sync sample box (§8.6.2). It lists the 1-based indices
// of samples that are sync points — for AV1 these are keyframes that
// can be decoded without any prior reference. When stss is absent,
// every sample is a sync sample (i.e. intra-only bitstream).
type Stss struct {
	FullBoxHeader
	// SampleNumbers are 1-based indices of sync samples, sorted
	// ascending.
	SampleNumbers []uint32
}

func (*Stss) BoxType() FourCC { return TypeStss }

func (s *Stss) MarshalPayload() ([]byte, error) {
	return nil, fmt.Errorf("%w: Stss.MarshalPayload not implemented", ErrInvalid)
}

func ParseStss(payload []byte) (*Stss, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	n := c.readU32()
	s := &Stss{FullBoxHeader: fbh, SampleNumbers: make([]uint32, 0, n)}
	for i := uint32(0); i < n; i++ {
		s.SampleNumbers = append(s.SampleNumbers, c.readU32())
	}
	if c.err != nil {
		return nil, c.err
	}
	return s, nil
}

// IsSync reports whether the 1-based sampleIndex is a sync sample.
func (s *Stss) IsSync(sampleIndex uint32) bool {
	if s == nil {
		return true // absent stss: every sample is a sync sample
	}
	// SampleNumbers is sorted ascending; linear scan is fine for the
	// small tables AVIS typically uses (< 100 keyframes).
	for _, n := range s.SampleNumbers {
		if n == sampleIndex {
			return true
		}
		if n > sampleIndex {
			return false
		}
	}
	return false
}

// Co64 is the 64-bit chunk offset box — same role as stco for large
// files.
type Co64 struct {
	FullBoxHeader
	Offsets []uint64
}

func (*Co64) BoxType() FourCC { return TypeCo64 }

func (s *Co64) MarshalPayload() ([]byte, error) {
	return nil, fmt.Errorf("%w: Co64.MarshalPayload not implemented", ErrInvalid)
}

func ParseCo64(payload []byte) (*Co64, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	n := c.readU32()
	s := &Co64{FullBoxHeader: fbh, Offsets: make([]uint64, 0, n)}
	for i := uint32(0); i < n; i++ {
		s.Offsets = append(s.Offsets, c.readU64())
	}
	if c.err != nil {
		return nil, c.err
	}
	return s, nil
}
