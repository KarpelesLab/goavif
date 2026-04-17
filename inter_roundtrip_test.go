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
