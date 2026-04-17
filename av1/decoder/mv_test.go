package decoder

import (
	"bytes"
	"testing"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// TestReadMVZeroJoint exercises the MV reader on a bitstream produced
// by encoding an MV_JOINT_ZERO symbol: the resulting MV should be
// (0, 0) with no components read.
func TestReadMVZeroJoint(t *testing.T) {
	var enc entropy.Encoder
	enc.Init(false)
	jcdf := append(cdfs.CDF(nil), cdfs.DefaultMvJointCDF...)
	enc.EncodeSymbol(jcdf, int(MVJointZero))
	buf := enc.Finish()

	var dec entropy.Decoder
	if err := dec.Init(buf, len(buf), false); err != nil {
		t.Fatalf("init: %v", err)
	}
	md := InitMVDecoder(&dec, false)
	mv := md.ReadMV()
	if mv.Row != 0 || mv.Col != 0 {
		t.Fatalf("MV_JOINT_ZERO decoded to (%d, %d), want (0, 0)", mv.Row, mv.Col)
	}
}

// TestReadMVPositiveClass0 verifies a small positive MV round-trips.
// Class 0, bit=1, fr=0, hp=1 → mag = 1*8 + 0 + 1 + 1 = 10 (eighth-pel).
func TestReadMVPositiveClass0(t *testing.T) {
	var enc entropy.Encoder
	enc.Init(false)
	jcdf := append(cdfs.CDF(nil), cdfs.DefaultMvJointCDF...)
	enc.EncodeSymbol(jcdf, int(MVJointHNZVZ))
	// Emit symbols that readComponent(0) will consume:
	//   sign=0, class=0, class0bit=1, class0fr=0, (hp skipped since allowHighPrecMV=false)
	enc.EncodeSymbol(append(cdfs.CDF(nil), cdfs.DefaultMvSignCDF[0]...), 0)
	enc.EncodeSymbol(append(cdfs.CDF(nil), cdfs.DefaultMvClassCDF[0]...), 0)
	enc.EncodeSymbol(append(cdfs.CDF(nil), cdfs.DefaultMvClass0BitCDF[0]...), 1)
	enc.EncodeSymbol(append(cdfs.CDF(nil), cdfs.DefaultMvClass0FrCDF[0][1]...), 0)
	buf := enc.Finish()

	var dec entropy.Decoder
	if err := dec.Init(buf, len(buf), false); err != nil {
		t.Fatalf("init: %v", err)
	}
	md := InitMVDecoder(&dec, false)
	mv := md.ReadMV()
	if mv.Row != 0 {
		t.Fatalf("Row = %d, want 0", mv.Row)
	}
	// With allowHighPrecMV=false, hp is forced to 1.
	// Magnitude = 1*8 + 0*2 + 1 + 1 = 10.
	wantCol := int32(10)
	if mv.Col != wantCol {
		t.Fatalf("Col = %d, want %d", mv.Col, wantCol)
	}
}

// TestDecodeWithRefIntraEquivalent verifies DecodeWithRef produces
// the same output as Decode for intra-only content. The ref
// argument must be ignored when the frame type is key.
func TestDecodeWithRefIntraEquivalent(t *testing.T) {
	// Minimal intra bitstream: 64×64 seq header + keyframe with a
	// single skip SB. Build via existing encoder APIs.
	seq := obu.WriteSequenceHeader(64, 64)
	fh := obu.WriteKeyFrameHeader(64, 64, 32)
	seqOBU := obu.WrapOBU(1, seq)
	frame := obu.WrapOBU(6, append(append([]byte(nil), fh...), byte(0), byte(0)))
	item := append(append([]byte(nil), seqOBU...), frame...)

	shParsed, err := obu.ParseSequenceHeader(seq)
	if err != nil {
		t.Fatalf("seq parse: %v", err)
	}

	// Bogus ref — since the frame is intra, DecodeWithRef must not
	// touch it.
	dummy := &Frame{}

	fA, errA := Decode(item, shParsed)
	fB, errB := DecodeWithRef(item, shParsed, dummy)
	// Both paths share the same codepath; they should report the
	// same outcome.
	if (errA == nil) != (errB == nil) {
		t.Fatalf("Decode err=%v vs DecodeWithRef err=%v diverge", errA, errB)
	}
	if errA != nil {
		return
	}
	if !bytes.Equal(fA.Y, fB.Y) {
		t.Fatalf("DecodeWithRef produced different luma than Decode")
	}
}

// TestReadMVNegativeClass1 verifies a larger negative MV round-trips.
func TestReadMVNegativeClass1(t *testing.T) {
	var enc entropy.Encoder
	enc.Init(false)
	jcdf := append(cdfs.CDF(nil), cdfs.DefaultMvJointCDF...)
	enc.EncodeSymbol(jcdf, int(MVJointHZVNZ))
	// Row component: sign=1 (negative), class=1, bits[0]=0, fr=1, (no hp)
	enc.EncodeSymbol(append(cdfs.CDF(nil), cdfs.DefaultMvSignCDF[1]...), 1)
	enc.EncodeSymbol(append(cdfs.CDF(nil), cdfs.DefaultMvClassCDF[1]...), 1)
	enc.EncodeSymbol(append(cdfs.CDF(nil), cdfs.DefaultMvBitsCDF[1][0]...), 0)
	enc.EncodeSymbol(append(cdfs.CDF(nil), cdfs.DefaultMvFrCDF[1]...), 1)
	buf := enc.Finish()

	var dec entropy.Decoder
	if err := dec.Init(buf, len(buf), false); err != nil {
		t.Fatalf("init: %v", err)
	}
	md := InitMVDecoder(&dec, false)
	mv := md.ReadMV()
	if mv.Col != 0 {
		t.Fatalf("Col = %d, want 0", mv.Col)
	}
	// Class 1: magInt = (1<<3) = 8 eighth-pel.
	// Add fr=1 (*2) = 2, hp=1, +1 offset = 8 + 2 + 1 + 1 = 12.
	// Sign=1 → negative.
	wantRow := int32(-12)
	if mv.Row != wantRow {
		t.Fatalf("Row = %d, want %d", mv.Row, wantRow)
	}
}
