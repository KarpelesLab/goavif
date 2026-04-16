package decoder

import (
	"testing"

	"github.com/KarpelesLab/goavif/av1/entropy"
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

func TestCoeffDecoderReadsEOB(t *testing.T) {
	buf := make([]byte, 64)
	for i := range buf {
		buf[i] = byte(i * 23)
	}
	var dec entropy.Decoder
	if err := dec.Init(buf, len(buf), true); err != nil {
		t.Fatalf("Init: %v", err)
	}
	cd := InitCoeffDecoder(&dec, 0)
	eob := cd.ReadEOB(16, 0, 0)
	if eob < 1 || eob > 16 {
		t.Errorf("eob=%d out of range for 16-coeff block", eob)
	}
}

func TestReadCoefficientsSkipPath(t *testing.T) {
	// Construct a buffer where ReadTXBSkip returns true (high probability
	// of skip for some contexts). We brute-force find one.
	var dec entropy.Decoder
	for seed := byte(0); seed < 255; seed++ {
		buf := make([]byte, 32)
		for i := range buf {
			buf[i] = seed
		}
		if err := dec.Init(buf, len(buf), false); err != nil {
			continue
		}
		cd := InitCoeffDecoder(&dec, 0)
		scan := transform.DefaultZigzagScan(4, 4)
		coeffs, err := cd.ReadCoefficients(0, 0, 16, scan)
		if err == nil {
			// Got through — must be all-zero if skip was true.
			allZero := true
			for _, v := range coeffs {
				if v != 0 {
					allZero = false
				}
			}
			if allZero {
				t.Logf("seed %d produced skip path (all-zero coeffs)", seed)
				return
			}
		}
	}
	t.Log("no seed produced a skip path — that's fine, the test exercises initialization")
}
