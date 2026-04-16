package goavif

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeAllRejectsNonAvifContainer(t *testing.T) {
	// Not a valid ISOBMFF file — ParseContainer returns an error.
	_, _, err := DecodeAll(strings.NewReader("this is not an isobmff file, but close enough to reach ParseContainer"))
	if err == nil {
		t.Fatal("expected error on garbage input")
	}
}

func TestDecodeAllEmptyInputReturnsError(t *testing.T) {
	_, _, err := DecodeAll(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("expected error on empty input")
	}
}
