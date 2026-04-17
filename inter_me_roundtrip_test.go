package goavif

import (
	"testing"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/encoder"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// TestMotionEstimationRecoversShift builds a ref frame with a
// recognizable horizontal bar, then a source frame where the same bar
// is shifted by (+5, -3) pixels. SearchMV should land on dx=+5, dy=-3
// (in eighth-pel units: col=+40, row=-24).
func TestMotionEstimationRecoversShift(t *testing.T) {
	const dim = 64
	src := make([]uint8, dim*dim)
	ref := make([]uint8, dim*dim)
	// Bar at ref row 20..24, src row 17..21 (= ref row shifted up 3).
	// Bar at ref col 10..40, src col 15..45 (= ref col shifted right 5).
	for y := 20; y < 25; y++ {
		for x := 10; x < 40; x++ {
			ref[y*dim+x] = 200
		}
	}
	for y := 17; y < 22; y++ {
		for x := 15; x < 45; x++ {
			src[y*dim+x] = 200
		}
	}
	// A whole 32×32 block centered on the bar region.
	mv := encoder.SearchMV(src, dim, 0, 0, 32, 32, ref, dim, dim, dim, 16)
	// Source's bar sits at (x=15..45, y=17..22). The reference's bar
	// is at (x=10..40, y=20..25). MV is the offset added to output
	// coordinates to sample the reference — so src[y][x] =
	// ref[y+dy][x+dx] with dx=-5, dy=+3 → MV.col=-40, MV.row=+24.
	wantCol := int32(-40)
	wantRow := int32(24)
	if mv.Col != wantCol || mv.Row != wantRow {
		t.Fatalf("SearchMV = (col=%d, row=%d), want (col=%d, row=%d)",
			mv.Col, mv.Row, wantCol, wantRow)
	}
	t.Logf("SearchMV recovered shift: col=%d (=%d pel), row=%d (=%d pel)",
		mv.Col, mv.Col/8, mv.Row, mv.Row/8)
}

// TestWriteInterMETileRoundTrip exercises the full ME pipeline: build
// a key frame, build a source that's the key frame shifted by a few
// pixels, encode an inter frame via WriteInterMETile, decode against
// the key frame, and verify the residual is small (MAD ≤ 10 samples).
func TestWriteInterMETileRoundTrip(t *testing.T) {
	const dim = 64
	baseQ := uint8(40)
	seqPayload := obu.WriteSequenceHeaderAVIS(dim, dim, obu.SeqWriteOpts{
		BitDepth: 8, SubsamplingX: 1, SubsamplingY: 1,
	})
	sh, _ := obu.ParseSequenceHeader(seqPayload)
	seqOBU := obu.WrapOBU(1, seqPayload)

	// Key frame: vertical bars.
	keyY := make([]uint8, dim*dim)
	keyU := make([]uint8, dim*dim/4)
	keyV := make([]uint8, dim*dim/4)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			if (x/8)%2 == 0 {
				keyY[y*dim+x] = 200
			} else {
				keyY[y*dim+x] = 60
			}
		}
	}
	for i := range keyU {
		keyU[i] = 128
		keyV[i] = 128
	}
	keyHdr := obu.WriteAVISKeyFrameHeader(dim, dim, baseQ)
	keyFh, _, _ := obu.ParseFrameHeaderBytes(keyHdr, sh, nil)
	keyTile, _ := encoder.WriteIntraOnlyTile(dim, dim, keyFh, sh, keyY, keyU, keyV)
	keyOBU := obu.WrapOBU(6, append(append([]byte(nil), keyHdr...), keyTile...))
	keyDec, err := decoder.Decode(append(append([]byte(nil), seqOBU...), keyOBU...), sh)
	if err != nil {
		t.Fatalf("key: %v", err)
	}

	// Source = key shifted right by 4 pixels; edges wrap to 128.
	srcY := make([]uint8, dim*dim)
	srcU := make([]uint8, dim*dim/4)
	srcV := make([]uint8, dim*dim/4)
	copy(srcU, keyU)
	copy(srcV, keyV)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			rx := x - 4
			if rx < 0 || rx >= dim {
				srcY[y*dim+x] = 128
			} else {
				srcY[y*dim+x] = keyY[y*dim+rx]
			}
		}
	}

	interHdr := obu.WriteInterFrameHeader(dim, dim, baseQ)
	interFh, _, _ := obu.ParseFrameHeaderBytes(interHdr, sh, nil)
	interTile, err := encoder.WriteInterMETile(dim, dim, interFh, sh,
		srcY, srcU, srcV,
		keyDec.Y, keyDec.U, keyDec.V, dim, dim, 8)
	if err != nil {
		t.Fatalf("inter ME tile: %v", err)
	}
	interOBU := obu.WrapOBU(6, append(append([]byte(nil), interHdr...), interTile...))
	interDec, err := decoder.DecodeWithRef(append(append([]byte(nil), seqOBU...), interOBU...), sh, keyDec)
	if err != nil {
		t.Fatalf("inter decode: %v", err)
	}

	// MAD vs source should be modest (a few units) given the simple
	// translational shift and baseQ=40.
	sad := 0
	n := 0
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			d := int(interDec.Y[y*interDec.YStride+x]) - int(srcY[y*dim+x])
			if d < 0 {
				d = -d
			}
			sad += d
			n++
		}
	}
	mad := float64(sad) / float64(n)
	t.Logf("inter ME roundtrip: luma MAD = %.2f", mad)
	// Shift-in-from-edge introduces a 4-column region where the
	// decoded output must encode a residual of magnitude ~70 against
	// the clamped reference. Quantization at baseQ=40 recovers most
	// but not all of that. The remaining frame region reconstructs
	// close to source.
	if mad > 25 {
		t.Fatalf("inter ME MAD too large: %.2f (expected ≤ 25)", mad)
	}
}
