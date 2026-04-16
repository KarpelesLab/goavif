package obu

import "testing"

func TestWriteKeyFrameHeaderRoundTrip(t *testing.T) {
	shPayload := WriteSequenceHeader(320, 240)
	sh, err := ParseSequenceHeader(shPayload)
	if err != nil {
		t.Fatalf("seq header: %v", err)
	}
	fhPayload := WriteKeyFrameHeader(320, 240, 32)
	fh, _, err := ParseFrameHeaderBytes(fhPayload, sh, nil)
	if err != nil {
		t.Fatalf("ParseFrameHeaderBytes: %v", err)
	}
	if fh.FrameType != KeyFrame {
		t.Fatalf("FrameType = %d want KEY_FRAME", fh.FrameType)
	}
	if !fh.ShowFrame {
		t.Fatal("ShowFrame should be true")
	}
	if !fh.ErrorResilientMode {
		t.Fatal("ErrorResilientMode should be true (KeyFrame + ShowFrame)")
	}
	if fh.Quant.BaseQIndex != 32 {
		t.Fatalf("BaseQIndex = %d want 32", fh.Quant.BaseQIndex)
	}
	if fh.LoopFilter.LevelY0 != 0 {
		t.Fatalf("LoopFilter.LevelY0 = %d want 0", fh.LoopFilter.LevelY0)
	}
}

func TestWriteKeyFrameHeaderZeroQIndex320x240(t *testing.T) {
	shPayload := WriteSequenceHeader(320, 240)
	sh, err := ParseSequenceHeader(shPayload)
	if err != nil {
		t.Fatalf("seq header: %v", err)
	}
	fhPayload := WriteKeyFrameHeader(320, 240, 0)
	fh, _, err := ParseFrameHeaderBytes(fhPayload, sh, nil)
	if err != nil {
		t.Fatalf("ParseFrameHeaderBytes: %v", err)
	}
	if fh.Quant.BaseQIndex != 0 {
		t.Fatalf("BaseQIndex = %d want 0", fh.Quant.BaseQIndex)
	}
}

func TestWriteKeyFrameHeaderSmall(t *testing.T) {
	shPayload := WriteSequenceHeader(64, 64)
	sh, err := ParseSequenceHeader(shPayload)
	if err != nil {
		t.Fatalf("seq header: %v", err)
	}
	fhPayload := WriteKeyFrameHeader(64, 64, 32)
	fh, _, err := ParseFrameHeaderBytes(fhPayload, sh, nil)
	if err != nil {
		t.Fatalf("ParseFrameHeaderBytes: %v", err)
	}
	if fh.Quant.BaseQIndex != 32 {
		t.Fatalf("BaseQIndex = %d want 32", fh.Quant.BaseQIndex)
	}
}
