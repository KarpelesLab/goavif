package isobmff

import (
	"encoding/binary"
	"testing"
)

func TestParseContainerHugePayloadLenNoPanic(t *testing.T) {
	// 16-byte box header: 32-bit size==1 (largesize follows), type "ftyp",
	// 64-bit largesize = 1<<63. PayloadLen() = largesize-16 which exceeds
	// MaxInt64, so the int64 conversion at container.go:45 wraps negative and
	// make([]byte, plen) panics with "makeslice: len out of range".
	data := make([]byte, 16)
	binary.BigEndian.PutUint32(data[0:4], 1)
	copy(data[4:8], "ftyp")
	binary.BigEndian.PutUint64(data[8:16], 1<<63)

	_, err := ParseContainer(data)
	if err == nil {
		t.Fatal("expected error for oversized payload, got nil")
	}
}
