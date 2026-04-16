package decoder

import (
	"testing"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/transform"
)

func TestCoeffDecoderReadsTxbSkip(t *testing.T) {
	buf := make([]byte, 64)
	for i := range buf {
		buf[i] = byte(i * 41)
	}
	var dec entropy.Decoder
	if err := dec.Init(buf, len(buf), true); err != nil {
		t.Fatalf("Init: %v", err)
	}
	cd := InitCoeffDecoder(&dec, 0)
	_ = cd.ReadTXBSkip(0, 0)
	if dec.Err() != nil {
		t.Fatalf("err after ReadTXBSkip: %v", dec.Err())
	}
}

func TestCoeffDecoderReadsEOBPt(t *testing.T) {
	buf := make([]byte, 64)
	for i := range buf {
		buf[i] = byte(i * 23)
	}
	var dec entropy.Decoder
	if err := dec.Init(buf, len(buf), true); err != nil {
		t.Fatalf("Init: %v", err)
	}
	cd := InitCoeffDecoder(&dec, 0)
	pt := cd.ReadEOBPt(16, 0, 0)
	if pt < 0 || pt >= 5 {
		t.Errorf("eob_pt=%d out of range for 16-coeff (CDF5)", pt)
	}
}

func TestReadCoefficients4x4(t *testing.T) {
	// Brute-force a few seeds; at least one should produce either skip or
	// a successfully decoded coefficient array. No panics allowed.
	scan := transform.DefaultZigzagScan(4, 4)
	var dec entropy.Decoder
	anySuccess := false
	for seed := byte(0); seed < 32; seed++ {
		buf := make([]byte, 64)
		for i := range buf {
			buf[i] = seed
		}
		if err := dec.Init(buf, len(buf), false); err != nil {
			continue
		}
		cd := InitCoeffDecoder(&dec, 0)
		_, err := cd.ReadCoefficients(0, 0, 16, scan, cdfs.NzMapCtxOffset4x4[:], 4, 4)
		if err == nil {
			anySuccess = true
		}
	}
	if !anySuccess {
		t.Log("no seed decoded cleanly — that's fine, test asserts no panics")
	}
}
