package isobmff

import (
	"bytes"
	"testing"
)

func TestFourCCRoundtrip(t *testing.T) {
	f := FourCCOf("av01")
	if f.String() != "av01" {
		t.Fatalf("String() = %q, want av01", f.String())
	}
	if !f.Equal("av01") {
		t.Fatalf("Equal(av01) = false")
	}
	if f.Equal("avif") {
		t.Fatalf("Equal(avif) = true; want false")
	}
}

func TestFtypRoundtrip(t *testing.T) {
	ft := &Ftyp{
		MajorBrand:   FourCCOf("avif"),
		MinorVersion: 0,
		CompatibleBrands: []FourCC{
			FourCCOf("avif"),
			FourCCOf("mif1"),
			FourCCOf("miaf"),
		},
	}
	var buf bytes.Buffer
	if err := writeBox(&buf, ft); err != nil {
		t.Fatalf("writeBox: %v", err)
	}
	got, err := ReadBoxes(&buf)
	if err != nil {
		t.Fatalf("ReadBoxes: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d boxes, want 1", len(got))
	}
	rb := got[0].(*RawBox)
	if rb.Type != TypeFtyp {
		t.Fatalf("type = %q, want ftyp", rb.Type)
	}
	parsed, err := ParseFtyp(rb.Payload)
	if err != nil {
		t.Fatalf("ParseFtyp: %v", err)
	}
	if parsed.MajorBrand != ft.MajorBrand {
		t.Errorf("major %q want %q", parsed.MajorBrand, ft.MajorBrand)
	}
	if len(parsed.CompatibleBrands) != len(ft.CompatibleBrands) {
		t.Errorf("compat count %d want %d", len(parsed.CompatibleBrands), len(ft.CompatibleBrands))
	}
	for i := range ft.CompatibleBrands {
		if parsed.CompatibleBrands[i] != ft.CompatibleBrands[i] {
			t.Errorf("compat[%d] = %q want %q", i, parsed.CompatibleBrands[i], ft.CompatibleBrands[i])
		}
	}
}

func TestHeaderLargeSize(t *testing.T) {
	// Build a payload whose total box size exceeds 4 GiB — we want the
	// largesize form to be selected without actually allocating 4 GiB.
	// We simulate by constructing a header manually.
	payload := []byte{1, 2, 3, 4}
	h := Header{
		Size:      uint64(1<<32) + 100, // > 4 GiB, forces largesize
		Type:      TypeMdat,
		HeaderLen: 16,
	}
	var buf bytes.Buffer
	if err := writeHeader(&buf, h); err != nil {
		t.Fatalf("writeHeader: %v", err)
	}
	if buf.Len() != 16 {
		t.Fatalf("large header len %d, want 16", buf.Len())
	}
	// First 4 bytes should be 1 (largesize marker).
	if buf.Bytes()[0] != 0 || buf.Bytes()[1] != 0 || buf.Bytes()[2] != 0 || buf.Bytes()[3] != 1 {
		t.Fatalf("first 4 bytes %x; want 00000001 (largesize marker)", buf.Bytes()[:4])
	}
	// Roundtrip just the header.
	r := bytes.NewReader(buf.Bytes())
	got, err := readHeader(r)
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	if got.Size != h.Size || got.Type != h.Type {
		t.Fatalf("header roundtrip: got %+v want %+v", got, h)
	}
	if got.HeaderLen != 16 {
		t.Fatalf("header len %d, want 16", got.HeaderLen)
	}
	// The payload bytes are not part of the header test; just confirm none were consumed.
	if len(payload) != 4 {
		t.Fatalf("payload clobbered")
	}
}

func TestIlocRoundtrip(t *testing.T) {
	il := &Iloc{
		FullBoxHeader:  FullBoxHeader{Version: 0},
		OffsetSize:     4,
		LengthSize:     4,
		BaseOffsetSize: 0,
		Items: []IlocItem{{
			ItemID: 1,
			Extents: []IlocExtent{{
				Offset: 0x1234,
				Length: 1000,
			}},
		}},
	}
	payload, err := il.MarshalPayload()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := ParseIloc(payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].ItemID != 1 {
		t.Fatalf("items: %+v", got.Items)
	}
	if got.Items[0].Extents[0].Offset != 0x1234 {
		t.Errorf("offset %d, want 0x1234", got.Items[0].Extents[0].Offset)
	}
	if got.Items[0].Extents[0].Length != 1000 {
		t.Errorf("length %d, want 1000", got.Items[0].Extents[0].Length)
	}
}

func TestIpmaRoundtrip(t *testing.T) {
	m := &Ipma{
		FullBoxHeader: FullBoxHeader{Version: 0, Flags: 0},
		Entries: []IpmaEntry{{
			ItemID: 1,
			Associations: []IpmaAssoc{
				{PropertyIndex: 1, Essential: false},
				{PropertyIndex: 2, Essential: true},
				{PropertyIndex: 3, Essential: false},
			},
		}},
	}
	payload, err := m.MarshalPayload()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := ParseIpma(payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("entries: %d", len(got.Entries))
	}
	if got.Entries[0].ItemID != 1 {
		t.Errorf("itemID: %d", got.Entries[0].ItemID)
	}
	for i, a := range got.Entries[0].Associations {
		if a != m.Entries[0].Associations[i] {
			t.Errorf("assoc[%d] = %+v want %+v", i, a, m.Entries[0].Associations[i])
		}
	}
}

func TestAv1CRoundtrip(t *testing.T) {
	c := &Av1C{
		SeqProfile:                      1,
		SeqLevelIdx0:                    13,
		SeqTier0:                        1,
		HighBitdepth:                    1,
		TwelveBit:                       0,
		Monochrome:                      0,
		ChromaSubsamplingX:              1,
		ChromaSubsamplingY:              1,
		ChromaSamplePosition:            2,
		InitialPresentationDelayPresent: 1,
		InitialPresentationDelayMinusOne: 5,
		ConfigOBUs:                      []byte{0x0a, 0x0b, 0x0c},
	}
	payload, err := c.MarshalPayload()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := ParseAv1C(payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.SeqProfile != c.SeqProfile ||
		got.SeqLevelIdx0 != c.SeqLevelIdx0 ||
		got.SeqTier0 != c.SeqTier0 ||
		got.HighBitdepth != c.HighBitdepth ||
		got.TwelveBit != c.TwelveBit ||
		got.Monochrome != c.Monochrome ||
		got.ChromaSubsamplingX != c.ChromaSubsamplingX ||
		got.ChromaSubsamplingY != c.ChromaSubsamplingY ||
		got.ChromaSamplePosition != c.ChromaSamplePosition ||
		got.InitialPresentationDelayPresent != c.InitialPresentationDelayPresent ||
		got.InitialPresentationDelayMinusOne != c.InitialPresentationDelayMinusOne {
		t.Errorf("fields mismatch:\n got %+v\nwant %+v", got, c)
	}
	if !bytes.Equal(got.ConfigOBUs, c.ConfigOBUs) {
		t.Errorf("ConfigOBUs = %x want %x", got.ConfigOBUs, c.ConfigOBUs)
	}
}

func TestColrNclxRoundtrip(t *testing.T) {
	c := &Colr{
		Type:                    ColrTypeNCLX,
		ColourPrimaries:         1,
		TransferCharacteristics: 13,
		MatrixCoefficients:      1,
		FullRange:               true,
	}
	payload, err := c.MarshalPayload()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := ParseColr(payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Type != c.Type ||
		got.ColourPrimaries != c.ColourPrimaries ||
		got.TransferCharacteristics != c.TransferCharacteristics ||
		got.MatrixCoefficients != c.MatrixCoefficients ||
		got.FullRange != c.FullRange ||
		!bytes.Equal(got.ICC, c.ICC) {
		t.Errorf("got %+v want %+v", got, c)
	}
}

func TestIrefRoundtrip(t *testing.T) {
	r := &Iref{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Entries: []IrefEntry{
			{Type: TypeAuxl, FromID: 2, ToIDs: []uint32{1}},
		},
	}
	payload, err := r.MarshalPayload()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := ParseIref(payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("entries: %d", len(got.Entries))
	}
	if got.Entries[0].Type != TypeAuxl ||
		got.Entries[0].FromID != 2 ||
		len(got.Entries[0].ToIDs) != 1 ||
		got.Entries[0].ToIDs[0] != 1 {
		t.Errorf("entry %+v", got.Entries[0])
	}
}
