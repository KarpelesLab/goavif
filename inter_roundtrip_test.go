package goavif

import (
	"bytes"
	"testing"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/encoder"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// TestInterCopyFrameRoundTrip proves the end-to-end inter path:
// encode a seq + key frame, then an inter frame made entirely of
// zero-MV skip blocks. The decoder should reproduce the key frame
// pixel-for-pixel since the inter frame is a pure copy.
func TestInterCopyFrameRoundTrip(t *testing.T) {
	const dim = 64
	baseQ := uint8(40)

	// Seq header for AVIS — non-reduced so inter frames are legal.
	seqPayload := obu.WriteSequenceHeaderAVIS(dim, dim, obu.SeqWriteOpts{
		BitDepth: 8, SubsamplingX: 1, SubsamplingY: 1,
	})
	sh, err := obu.ParseSequenceHeader(seqPayload)
	if err != nil {
		t.Fatalf("seq parse: %v", err)
	}

	// Key frame under AVIS-mode sequence header.
	keyHdr := obu.WriteAVISKeyFrameHeader(dim, dim, baseQ)
	keyFh, _, err := obu.ParseFrameHeaderBytes(keyHdr, sh, nil)
	if err != nil {
		t.Fatalf("key header parse under AVIS seq: %v", err)
	}

	// Encode a synthetic luma gradient as a keyframe.
	srcY := make([]uint8, dim*dim)
	srcU := make([]uint8, dim*dim/4)
	srcV := make([]uint8, dim*dim/4)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			srcY[y*dim+x] = uint8(x * 4)
		}
	}
	for i := range srcU {
		srcU[i] = 128
		srcV[i] = 128
	}
	keyTile, err := encoder.WriteIntraOnlyTile(dim, dim, keyFh, sh, srcY, srcU, srcV)
	if err != nil {
		t.Fatalf("key tile: %v", err)
	}
	seqOBU := obu.WrapOBU(1, seqPayload)
	keyFrameOBU := obu.WrapOBU(6, append(append([]byte(nil), keyHdr...), keyTile...))

	// Decode the key frame.
	keySample := append(append([]byte(nil), seqOBU...), keyFrameOBU...)
	keyDecoded, err := decoder.Decode(keySample, sh)
	if err != nil {
		t.Fatalf("key decode: %v", err)
	}

	// Now encode the inter copy frame.
	interHdr := obu.WriteInterFrameHeader(dim, dim, baseQ)
	interFh, _, err := obu.ParseFrameHeaderBytes(interHdr, sh, nil)
	if err != nil {
		t.Fatalf("inter header parse: %v", err)
	}
	interTile, err := encoder.WriteInterCopyTile(dim, dim, interFh, sh)
	if err != nil {
		t.Fatalf("inter tile: %v", err)
	}
	interFrameOBU := obu.WrapOBU(6, append(append([]byte(nil), interHdr...), interTile...))
	interSample := append(append([]byte(nil), seqOBU...), interFrameOBU...)

	interDecoded, err := decoder.DecodeWithRef(interSample, sh, keyDecoded)
	if err != nil {
		t.Fatalf("inter decode: %v", err)
	}

	// The inter copy frame should decode to pixels identical to the
	// reference frame (within no drift since we skipped residuals
	// and used zero-MV motion compensation).
	if !bytes.Equal(keyDecoded.Y, interDecoded.Y) {
		// Count mismatches to report helpfully.
		mismatches := 0
		for i := range keyDecoded.Y {
			if keyDecoded.Y[i] != interDecoded.Y[i] {
				mismatches++
			}
		}
		t.Fatalf("inter copy frame luma diverges from ref: %d / %d samples mismatch",
			mismatches, len(keyDecoded.Y))
	}
}

// TestInterShiftedFrameRoundTrip proves the motion-vector path works
// end-to-end: encode an inter frame with a uniform integer-pel MV,
// decode, verify the decoded frame matches the reference shifted by
// that MV (with edge clamping). Exercises MV encoding → MV decoding
// → motion compensation.
func TestInterShiftedFrameRoundTrip(t *testing.T) {
	const dim = 64
	baseQ := uint8(40)

	seqPayload := obu.WriteSequenceHeaderAVIS(dim, dim, obu.SeqWriteOpts{
		BitDepth: 8, SubsamplingX: 1, SubsamplingY: 1,
	})
	sh, err := obu.ParseSequenceHeader(seqPayload)
	if err != nil {
		t.Fatalf("seq parse: %v", err)
	}
	seqOBU := obu.WrapOBU(1, seqPayload)

	// Encode a keyframe with a horizontal gradient so MV effects are
	// visible in luma.
	keyHdr := obu.WriteAVISKeyFrameHeader(dim, dim, baseQ)
	keyFh, _, err := obu.ParseFrameHeaderBytes(keyHdr, sh, nil)
	if err != nil {
		t.Fatalf("key fh: %v", err)
	}
	srcY := make([]uint8, dim*dim)
	srcU := make([]uint8, dim*dim/4)
	srcV := make([]uint8, dim*dim/4)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			srcY[y*dim+x] = uint8(x * 4)
		}
	}
	for i := range srcU {
		srcU[i] = 128
		srcV[i] = 128
	}
	keyTile, err := encoder.WriteIntraOnlyTile(dim, dim, keyFh, sh, srcY, srcU, srcV)
	if err != nil {
		t.Fatalf("key tile: %v", err)
	}
	keyOBU := obu.WrapOBU(6, append(append([]byte(nil), keyHdr...), keyTile...))
	keyDec, err := decoder.Decode(append(append([]byte(nil), seqOBU...), keyOBU...), sh)
	if err != nil {
		t.Fatalf("key decode: %v", err)
	}

	// Shift by +2 integer pels horizontally (col = 16 eighth-pel).
	shiftCol := int32(16)
	mv := decoder.MV{Row: 0, Col: shiftCol}

	interHdr := obu.WriteInterFrameHeader(dim, dim, baseQ)
	interFh, _, err := obu.ParseFrameHeaderBytes(interHdr, sh, nil)
	if err != nil {
		t.Fatalf("inter fh: %v", err)
	}
	interTile, err := encoder.WriteInterUniformMVTile(dim, dim, interFh, sh, mv)
	if err != nil {
		t.Fatalf("inter tile: %v", err)
	}
	interOBU := obu.WrapOBU(6, append(append([]byte(nil), interHdr...), interTile...))
	interSample := append(append([]byte(nil), seqOBU...), interOBU...)

	interDec, err := decoder.DecodeWithRef(interSample, sh, keyDec)
	if err != nil {
		t.Fatalf("inter decode: %v", err)
	}

	// Expected: reference shifted by +2 pixels right, with left edge
	// clamped to the leftmost source pixel. So decoded[y, x] should
	// equal keyDec.Y[y, x+2] (clamped to x+2 <= 63).
	mismatches := 0
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			want := keyDec.Y[y*keyDec.YStride+clampForTest(x+2, 0, dim-1)]
			got := interDec.Y[y*interDec.YStride+x]
			if got != want {
				mismatches++
				if mismatches < 5 {
					t.Logf("mismatch at (%d,%d): got %d, want %d", x, y, got, want)
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("shifted inter frame: %d / %d samples mismatch (MV=%+v)",
			mismatches, dim*dim, mv)
	}
}

// TestInterResidualRoundTrip encodes an inter frame whose source
// differs from the reference, exercising the coefficient residual
// path on top of motion compensation. MV is zero; the inter frame
// carries the full difference as a quantized Y + UV residual.
func TestInterResidualRoundTrip(t *testing.T) {
	const dim = 64
	baseQ := uint8(40)

	seqPayload := obu.WriteSequenceHeaderAVIS(dim, dim, obu.SeqWriteOpts{
		BitDepth: 8, SubsamplingX: 1, SubsamplingY: 1,
	})
	sh, err := obu.ParseSequenceHeader(seqPayload)
	if err != nil {
		t.Fatalf("seq: %v", err)
	}
	seqOBU := obu.WrapOBU(1, seqPayload)

	// Reference frame: horizontal gradient.
	refSrcY := make([]uint8, dim*dim)
	refSrcU := make([]uint8, dim*dim/4)
	refSrcV := make([]uint8, dim*dim/4)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			refSrcY[y*dim+x] = uint8(x * 4)
		}
	}
	for i := range refSrcU {
		refSrcU[i] = 128
		refSrcV[i] = 128
	}
	keyHdr := obu.WriteAVISKeyFrameHeader(dim, dim, baseQ)
	keyFh, _, _ := obu.ParseFrameHeaderBytes(keyHdr, sh, nil)
	keyTile, err := encoder.WriteIntraOnlyTile(dim, dim, keyFh, sh, refSrcY, refSrcU, refSrcV)
	if err != nil {
		t.Fatalf("key tile: %v", err)
	}
	keyOBU := obu.WrapOBU(6, append(append([]byte(nil), keyHdr...), keyTile...))
	keyDec, err := decoder.Decode(append(append([]byte(nil), seqOBU...), keyOBU...), sh)
	if err != nil {
		t.Fatalf("key decode: %v", err)
	}

	// Inter source: shifted gradient (so residual is non-zero).
	interSrcY := make([]uint8, dim*dim)
	interSrcU := make([]uint8, dim*dim/4)
	interSrcV := make([]uint8, dim*dim/4)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			// Brighten every pixel by 20 — small uniform delta.
			v := int(refSrcY[y*dim+x]) + 20
			if v > 255 {
				v = 255
			}
			interSrcY[y*dim+x] = uint8(v)
		}
	}
	for i := range interSrcU {
		interSrcU[i] = 128
		interSrcV[i] = 128
	}

	interHdr := obu.WriteInterFrameHeader(dim, dim, baseQ)
	interFh, _, _ := obu.ParseFrameHeaderBytes(interHdr, sh, nil)
	interTile, err := encoder.WriteInterResidualTile(dim, dim, interFh, sh,
		decoder.MV{Row: 0, Col: 0},
		interSrcY, interSrcU, interSrcV,
		keyDec.Y, keyDec.U, keyDec.V, dim, dim)
	if err != nil {
		t.Fatalf("inter tile: %v", err)
	}
	interOBU := obu.WrapOBU(6, append(append([]byte(nil), interHdr...), interTile...))
	interSample := append(append([]byte(nil), seqOBU...), interOBU...)

	interDec, err := decoder.DecodeWithRef(interSample, sh, keyDec)
	if err != nil {
		t.Fatalf("inter decode: %v", err)
	}

	// Measure average absolute error vs the intended source. The
	// residual path is lossy — coefs are quantized — so we allow
	// some drift. For baseQ=40 we expect drift < 20 per sample on
	// a smooth source.
	totalErr := 0
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			want := int(interSrcY[y*dim+x])
			got := int(interDec.Y[y*interDec.YStride+x])
			d := want - got
			if d < 0 {
				d = -d
			}
			totalErr += d
		}
	}
	mad := totalErr / (dim * dim)
	t.Logf("inter residual MAD vs source = %d", mad)
	if mad > 20 {
		t.Fatalf("inter residual diverged too far: MAD=%d", mad)
	}
}

// TestInterVerticalMVRoundTrip exercises the MV row component by
// shifting the reference down instead of right. Catches bugs in the
// per-component class-1 bit emission for the vertical MV CDF.
func TestInterVerticalMVRoundTrip(t *testing.T) {
	const dim = 64
	baseQ := uint8(40)

	seqPayload := obu.WriteSequenceHeaderAVIS(dim, dim, obu.SeqWriteOpts{
		BitDepth: 8, SubsamplingX: 1, SubsamplingY: 1,
	})
	sh, err := obu.ParseSequenceHeader(seqPayload)
	if err != nil {
		t.Fatalf("seq: %v", err)
	}
	seqOBU := obu.WrapOBU(1, seqPayload)

	keyHdr := obu.WriteAVISKeyFrameHeader(dim, dim, baseQ)
	keyFh, _, _ := obu.ParseFrameHeaderBytes(keyHdr, sh, nil)

	// Vertical gradient so row shifts are visible.
	srcY := make([]uint8, dim*dim)
	srcU := make([]uint8, dim*dim/4)
	srcV := make([]uint8, dim*dim/4)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			srcY[y*dim+x] = uint8(y * 4)
		}
	}
	for i := range srcU {
		srcU[i] = 128
		srcV[i] = 128
	}
	keyTile, _ := encoder.WriteIntraOnlyTile(dim, dim, keyFh, sh, srcY, srcU, srcV)
	keyOBU := obu.WrapOBU(6, append(append([]byte(nil), keyHdr...), keyTile...))
	keyDec, err := decoder.Decode(append(append([]byte(nil), seqOBU...), keyOBU...), sh)
	if err != nil {
		t.Fatalf("key: %v", err)
	}

	// Shift down by +3 integer pels (row = 24 eighth-pel).
	mv := decoder.MV{Row: 24, Col: 0}
	interHdr := obu.WriteInterFrameHeader(dim, dim, baseQ)
	interFh, _, _ := obu.ParseFrameHeaderBytes(interHdr, sh, nil)
	interTile, err := encoder.WriteInterUniformMVTile(dim, dim, interFh, sh, mv)
	if err != nil {
		t.Fatalf("inter tile: %v", err)
	}
	interOBU := obu.WrapOBU(6, append(append([]byte(nil), interHdr...), interTile...))
	interDec, err := decoder.DecodeWithRef(append(append([]byte(nil), seqOBU...), interOBU...), sh, keyDec)
	if err != nil {
		t.Fatalf("inter: %v", err)
	}

	mismatches := 0
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			want := keyDec.Y[clampForTest(y+3, 0, dim-1)*keyDec.YStride+x]
			got := interDec.Y[y*interDec.YStride+x]
			if got != want {
				mismatches++
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("vertical MV mismatch: %d / %d (MV=%+v)", mismatches, dim*dim, mv)
	}
}

// TestInterSubPelMVRoundTrip exercises sub-pel motion compensation
// through the full pipeline. The MV col=12 eighth-pel = 1.5 integer
// pel, so the 8-tap filter runs at phase 8 (mid-pel). For a smooth
// horizontal gradient the filtered output should still be monotonic
// and within tight bounds of the expected interpolated value.
func TestInterSubPelMVRoundTrip(t *testing.T) {
	const dim = 64
	baseQ := uint8(40)
	seqPayload := obu.WriteSequenceHeaderAVIS(dim, dim, obu.SeqWriteOpts{
		BitDepth: 8, SubsamplingX: 1, SubsamplingY: 1,
	})
	sh, _ := obu.ParseSequenceHeader(seqPayload)
	seqOBU := obu.WrapOBU(1, seqPayload)

	// Smooth horizontal gradient — luma proportional to x.
	srcY := make([]uint8, dim*dim)
	srcU := make([]uint8, dim*dim/4)
	srcV := make([]uint8, dim*dim/4)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			srcY[y*dim+x] = uint8(x * 4)
		}
	}
	for i := range srcU {
		srcU[i] = 128
		srcV[i] = 128
	}
	keyHdr := obu.WriteAVISKeyFrameHeader(dim, dim, baseQ)
	keyFh, _, _ := obu.ParseFrameHeaderBytes(keyHdr, sh, nil)
	keyTile, _ := encoder.WriteIntraOnlyTile(dim, dim, keyFh, sh, srcY, srcU, srcV)
	keyOBU := obu.WrapOBU(6, append(append([]byte(nil), keyHdr...), keyTile...))
	keyDec, err := decoder.Decode(append(append([]byte(nil), seqOBU...), keyOBU...), sh)
	if err != nil {
		t.Fatalf("key: %v", err)
	}

	// MV col=12 eighth-pel = 1 integer + phase 4. Phase×2 = 8 (mid)
	// after the eighth→sixteenth mapping in MotionCompensate.
	mv := decoder.MV{Row: 0, Col: 12}
	interHdr := obu.WriteInterFrameHeader(dim, dim, baseQ)
	interFh, _, _ := obu.ParseFrameHeaderBytes(interHdr, sh, nil)
	interTile, err := encoder.WriteInterUniformMVTile(dim, dim, interFh, sh, mv)
	if err != nil {
		t.Fatalf("inter tile: %v", err)
	}
	interOBU := obu.WrapOBU(6, append(append([]byte(nil), interHdr...), interTile...))
	interDec, err := decoder.DecodeWithRef(append(append([]byte(nil), seqOBU...), interOBU...), sh, keyDec)
	if err != nil {
		t.Fatalf("inter: %v", err)
	}

	// The decoded sub-pel shifted output should behave like the
	// reference shifted by ~1.5 pixels: the interpolated value at
	// output column x should land between reference columns x+1 and
	// x+2 (within some tolerance, accounting for ref quantization
	// drift and 8-tap filter ringing on slope changes).
	row := dim / 2
	betweenCount := 0
	for x := 10; x < 30; x++ {
		want1 := int(keyDec.Y[row*keyDec.YStride+x+1])
		want2 := int(keyDec.Y[row*keyDec.YStride+x+2])
		v := int(interDec.Y[row*interDec.YStride+x])
		lo, hi := want1, want2
		if hi < lo {
			lo, hi = hi, lo
		}
		// Tolerate ±6 around the expected band (quantization +
		// filter ringing).
		if v >= lo-6 && v <= hi+6 {
			betweenCount++
		}
	}
	t.Logf("sub-pel shift: %d/20 samples sit within ±6 of neighboring int-pel values", betweenCount)
	if betweenCount < 15 {
		t.Fatalf("sub-pel filter output too far from expected interpolated band: %d/20", betweenCount)
	}
}

func clampForTest(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
