package entropy

import (
	"testing"
)

// Encoder/decoder round-trip is not yet bit-exact — the current
// Encoder is a skeleton that primes the 15-bit initial state and
// mirrors the decoder's arithmetic, but without the deferred-carry
// propagation + per-renormalization high-bit emission that AV1's
// range coder needs. The tests below sanity-check that the
// Encoder produces some output and doesn't panic.

func TestEncoderInitAndFinishProduceBytes(t *testing.T) {
	var enc Encoder
	enc.Init(false)
	enc.EncodeBool(0, 16384)
	enc.EncodeBool(1, 8000)
	buf := enc.Finish()
	if len(buf) == 0 {
		t.Fatal("encoder produced empty buffer after two bool encodes + finish")
	}
}

func TestEncoderLiteralAccumulates(t *testing.T) {
	var enc Encoder
	enc.Init(false)
	enc.EncodeLiteral(0x5A, 8)
	enc.EncodeLiteral(0x1234, 16)
	if len(enc.Bytes()) == 0 {
		t.Fatal("literal encode emitted no bytes")
	}
}
