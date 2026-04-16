package goavif

import (
	"image"
	"strings"
	"testing"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/obu"
	"github.com/KarpelesLab/goavif/isobmff"
)

func TestFindAlphaItemIDReturnsZeroWithoutIref(t *testing.T) {
	ct := &isobmff.Container{Meta: &isobmff.Meta{}}
	if got := findAlphaItemID(ct, 1); got != 0 {
		t.Fatalf("no-iref case: expected 0, got %d", got)
	}
}

func TestFindAlphaItemIDMatchesAuxlPointingAtPrimary(t *testing.T) {
	iref := &isobmff.Iref{
		Entries: []isobmff.IrefEntry{
			{Type: isobmff.TypeAuxl, FromID: 42, ToIDs: []uint32{7}},
		},
	}
	// Build an iprp that associates itemID 42 with an AuxC carrying the alpha URN.
	aux := &isobmff.AuxC{AuxType: alphaURN}
	iprp := &isobmff.Iprp{
		Ipco: &isobmff.Ipco{
			Properties: []isobmff.Box{aux},
		},
		Ipma: []*isobmff.Ipma{
			{
				Entries: []isobmff.IpmaEntry{
					{
						ItemID: 42,
						Associations: []isobmff.IpmaAssoc{
							{PropertyIndex: 1},
						},
					},
				},
			},
		},
	}
	ct := &isobmff.Container{
		Meta: &isobmff.Meta{
			Children: []isobmff.Box{iref, iprp},
		},
	}
	if got := findAlphaItemID(ct, 7); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestFindAlphaItemIDRejectsNonAlphaAuxC(t *testing.T) {
	// Same shape but the AuxC URN isn't the alpha one.
	aux := &isobmff.AuxC{AuxType: "urn:mpeg:mpegB:cicp:systems:auxiliary:depth"}
	iprp := &isobmff.Iprp{
		Ipco: &isobmff.Ipco{Properties: []isobmff.Box{aux}},
		Ipma: []*isobmff.Ipma{
			{
				Entries: []isobmff.IpmaEntry{
					{
						ItemID:       42,
						Associations: []isobmff.IpmaAssoc{{PropertyIndex: 1}},
					},
				},
			},
		},
	}
	iref := &isobmff.Iref{
		Entries: []isobmff.IrefEntry{
			{Type: isobmff.TypeAuxl, FromID: 42, ToIDs: []uint32{7}},
		},
	}
	ct := &isobmff.Container{Meta: &isobmff.Meta{Children: []isobmff.Box{iref, iprp}}}
	if got := findAlphaItemID(ct, 7); got != 0 {
		t.Fatalf("non-alpha AuxC should return 0, got %d", got)
	}
}

func TestCompositeNRGBAMatchesSize(t *testing.T) {
	color := &decoder.Frame{
		Width: 2, Height: 2, BitDepth: 8,
		Y: []byte{0, 0, 0, 0},
		U: []byte{128}, V: []byte{128},
		Seq: &obu.SequenceHeader{},
	}
	color.Subsampling.X, color.Subsampling.Y = 1, 1
	alpha := &decoder.Frame{Width: 2, Height: 2, BitDepth: 8, Y: []byte{255, 128, 64, 0}}
	img, err := compositeNRGBA(color, alpha)
	if err != nil {
		t.Fatalf("composite: %v", err)
	}
	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatalf("expected *image.NRGBA, got %T", img)
	}
	got := []uint8{nrgba.Pix[3], nrgba.Pix[7], nrgba.Pix[11], nrgba.Pix[15]}
	want := []uint8{255, 128, 64, 0}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("alpha[%d] = %d want %d", i, got[i], want[i])
		}
	}
}

func TestCompositeNRGBADimensionMismatch(t *testing.T) {
	color := &decoder.Frame{Width: 2, Height: 2}
	alpha := &decoder.Frame{Width: 3, Height: 2}
	_, err := compositeNRGBA(color, alpha)
	if err == nil || !strings.Contains(err.Error(), "alpha size") {
		t.Fatalf("expected size-mismatch error, got %v", err)
	}
}

func TestCompositeNRGBA64With10BitAlpha(t *testing.T) {
	color := &decoder.Frame{
		Width: 2, Height: 2, BitDepth: 10,
		Y16: []uint16{512, 512, 512, 512},
		U16: []uint16{512}, V16: []uint16{512},
		Seq: &obu.SequenceHeader{},
	}
	color.Subsampling.X, color.Subsampling.Y = 1, 1
	color.Seq.Color.BitDepth = 10
	color.Seq.Color.SubsamplingX = 1
	color.Seq.Color.SubsamplingY = 1
	color.Seq.Color.ColorRange = true
	alpha := &decoder.Frame{
		Width: 2, Height: 2, BitDepth: 10,
		Y16: []uint16{1023, 512, 256, 0},
	}
	img, err := compositeNRGBA64(color, alpha)
	if err != nil {
		t.Fatalf("composite: %v", err)
	}
	nrgba, ok := img.(*image.NRGBA64)
	if !ok {
		t.Fatalf("expected *image.NRGBA64, got %T", img)
	}
	// Alpha at pixel 0: 1023 << (16-10) = 1023<<6 = 65472.
	a0 := (uint16(nrgba.Pix[6]) << 8) | uint16(nrgba.Pix[7])
	if a0 < 65400 {
		t.Fatalf("alpha[0] = %d, want ~65472", a0)
	}
	// Alpha at pixel 3: 0 << 6 = 0.
	a3 := (uint16(nrgba.Pix[8*3+6]) << 8) | uint16(nrgba.Pix[8*3+7])
	if a3 != 0 {
		t.Fatalf("alpha[3] = %d, want 0", a3)
	}
}
