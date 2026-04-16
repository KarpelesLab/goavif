package decoder

import (
	"testing"

	"github.com/KarpelesLab/goavif/av1/obu"
)

// TestHBDFilterDispatchRunsWithoutPanic exercises applyLoopFilter,
// applyCDEF, applyLoopRestoration and applyFilmGrain against a
// FrameState allocated with NewFrameStateHBD. The synthetic frame
// has uniform Y16/U16/V16 planes so the filter primitives must be
// no-op-equivalent; we only assert no panic and no overflow past
// the bit-depth max.
func TestHBDFilterDispatchRunsWithoutPanic(t *testing.T) {
	for _, bd := range []int{10, 12} {
		t.Run(bdName(bd), func(t *testing.T) {
			fs := NewFrameStateHBD(64, 64, 1, 1, false, bd)
			// Fill with a uniform mid-grey value.
			mid := uint16(1 << uint(bd-1))
			for i := range fs.Y16 {
				fs.Y16[i] = mid
			}
			for i := range fs.U16 {
				fs.U16[i] = mid
				fs.V16[i] = mid
			}

			sh := minimalSeqHeader()
			sh.EnableCdef = true
			sh.EnableRestoration = true

			fh := minimalFrameHeader()
			fh.LR.UsesLR = true
			fh.LR.FrameRestorationType[0] = obu.RestorationWiener
			fh.LR.Log2RestorationUnitSize[0] = 6

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("HBD filter pipeline panic: %v", r)
				}
			}()
			applyLoopFilter(fs, fh)
			applyCDEF(fs, fh, sh)
			applyLoopRestoration(fs, fh, sh)
			applyFilmGrain(fs, fh, sh)

			maxV := uint16((1 << uint(bd)) - 1)
			for i, v := range fs.Y16 {
				if v > maxV {
					t.Fatalf("Y16[%d]=%d exceeded %d-bit max", i, v, bd)
				}
			}
			for i, v := range fs.U16 {
				if v > maxV {
					t.Fatalf("U16[%d]=%d exceeded %d-bit max", i, v, bd)
				}
			}
			for i, v := range fs.V16 {
				if v > maxV {
					t.Fatalf("V16[%d]=%d exceeded %d-bit max", i, v, bd)
				}
			}
		})
	}
}

func bdName(bd int) string {
	switch bd {
	case 10:
		return "10bit"
	case 12:
		return "12bit"
	}
	return "unknown"
}

// TestHBDEndToEndTileDecode exercises the full 10/12-bit decode path:
// DecodeSuperblock walks partitions and lands predict + reconstruct
// into the Y16/U16/V16 planes, then every frame-level filter runs.
// Synthetic bitstream; we only assert no panic + in-range samples.
func TestHBDEndToEndTileDecode(t *testing.T) {
	for _, bd := range []int{10, 12} {
		t.Run(bdName(bd), func(t *testing.T) {
			sh := minimalSeqHeader()
			sh.Color.BitDepth = uint8(bd)

			fh := minimalFrameHeader()

			tileData := make([]byte, 256)
			for i := range tileData {
				tileData[i] = byte((i*17 + 3) & 0xFF)
			}

			td, err := NewTileDecoder(tileData, fh, sh)
			if err != nil {
				t.Fatalf("NewTileDecoder: %v", err)
			}
			fs := NewFrameStateHBD(
				int(fh.FrameWidth), int(fh.FrameHeight),
				int(sh.Color.SubsamplingX), int(sh.Color.SubsamplingY),
				sh.Color.Monochrome, bd,
			)

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("HBD pipeline panic: %v", r)
				}
			}()
			_ = td.DecodeSuperblock(fs, 0, 0)

			applyLoopFilter(fs, fh)
			applyCDEF(fs, fh, sh)
			applyLoopRestoration(fs, fh, sh)
			applyFilmGrain(fs, fh, sh)

			maxV := uint16((1 << uint(bd)) - 1)
			for i, v := range fs.Y16 {
				if v > maxV {
					t.Fatalf("Y16[%d]=%d exceeded %d-bit max", i, v, bd)
				}
			}
			for i, v := range fs.U16 {
				if v > maxV {
					t.Fatalf("U16[%d]=%d exceeded %d-bit max", i, v, bd)
				}
			}
		})
	}
}
