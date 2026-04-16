package decoder

import (
	"testing"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// TestPipelineDrivesEndToEndWithoutPanic walks the decoder's top-level
// entry points with controlled inputs to prove:
//
//   - NewTileDecoder initializes every CDF table and the coeff decoder
//   - DecodeSuperblock walks the partition tree through the full pipeline
//     (partition → mode → skip → predict → reconstruct → frame filters)
//   - applyLoopFilter / applyCDEF / applyLoopRestoration all run when
//     their respective enable flags are set, and all no-op cleanly
//     when flags are off
//
// It doesn't assert pixel-level output (a proper conformance test
// needs real AV1 vectors) — the goal is to catch pipeline-level
// regressions: nil derefs, out-of-bounds, infinite loops.
func TestPipelineDrivesEndToEndWithoutPanic(t *testing.T) {
	cases := []struct {
		name           string
		enableCdef     bool
		enableLR       bool
		segEnabled     bool
		sbSize128      bool
		expectDecodeOK bool
	}{
		{name: "bare", expectDecodeOK: true},
		{name: "cdef", enableCdef: true, expectDecodeOK: true},
		{name: "lr", enableLR: true, expectDecodeOK: true},
		{name: "cdef+lr", enableCdef: true, enableLR: true, expectDecodeOK: true},
		{name: "seg", segEnabled: true, expectDecodeOK: true},
		{name: "128sb", sbSize128: true, expectDecodeOK: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sh := minimalSeqHeader()
			sh.EnableCdef = c.enableCdef
			sh.EnableRestoration = c.enableLR
			sh.Use128x128Superblock = c.sbSize128

			fh := minimalFrameHeader()
			fh.Segmentation.Enabled = c.segEnabled
			fh.Segmentation.UpdateMap = c.segEnabled
			if c.enableLR {
				fh.LR.UsesLR = true
				fh.LR.FrameRestorationType[0] = obu.RestorationWiener
				fh.LR.Log2RestorationUnitSize[0] = 6 // 64
			}

			// Synthetic but structurally valid tile bitstream.
			tileData := make([]byte, 256)
			for i := range tileData {
				tileData[i] = byte((i*13 + 7) & 0xFF)
			}

			td, err := NewTileDecoder(tileData, fh, sh)
			if err != nil {
				t.Fatalf("NewTileDecoder: %v", err)
			}
			fs := NewFrameState(
				int(fh.FrameWidth), int(fh.FrameHeight),
				int(sh.Color.SubsamplingX), int(sh.Color.SubsamplingY),
				sh.Color.Monochrome,
			)

			// Run over the top-left superblock; we don't care about
			// pixel accuracy, only that the pipeline doesn't blow up.
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("pipeline panic: %v", r)
				}
			}()
			_ = td.DecodeSuperblock(fs, 0, 0)

			// Run the frame-level filters even if DecodeSuperblock
			// errored — they still need to handle the partial frame.
			applyLoopFilter(fs, fh)
			applyCDEF(fs, fh, sh)
			applyLoopRestoration(fs, fh, sh)
		})
	}
}

func TestCoeffDecoderReadsEveryCDFFamilyWithoutPanic(t *testing.T) {
	// Prime a coeff decoder with arbitrary bytes and call each reader
	// once, covering every CDF family copied into the struct.
	buf := make([]byte, 128)
	for i := range buf {
		buf[i] = byte((i * 29) & 0xFF)
	}
	var dec entropy.Decoder
	if err := dec.Init(buf, len(buf), false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	cd := InitCoeffDecoder(&dec, 0)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("coeff decoder panic: %v", r)
		}
	}()

	_ = cd.ReadTXBSkip(0, 0)
	_ = cd.ReadEOBPt(16, 0, 0)
	_ = cd.ReadEOBPt(32, 0, 0)
	_ = cd.ReadEOBPt(64, 0, 0)
	_ = cd.ReadEOBPt(128, 0, 0)
	_ = cd.ReadEOBPt(256, 0, 0)
	_ = cd.ReadEOBPt(512, 0, 0)
	_ = cd.ReadEOBPt(1024, 0, 0)
	_ = cd.ReadBaseLevel(0, 0, 0)
	_ = cd.ReadBaseLevelEOB(0, 0, 0)
	_ = cd.ReadBrLevel(0, 0, 0)
	_ = cd.ReadDCSign(0, 0)
	_ = cd.ReadUniformBit()
	_ = cd.ReadIntraTxType(1, 0, 0)
	_ = cd.ReadIntraTxType(2, 0, 0)
}
