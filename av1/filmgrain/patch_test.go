package filmgrain

import "testing"

func TestNewLumaTemplateDimensions(t *testing.T) {
	tpl := NewLumaTemplate(0x1234, 0, nil, 7)
	if tpl.Rows != 73 || tpl.Cols != 73 {
		t.Fatalf("luma template wrong shape: %dx%d", tpl.Rows, tpl.Cols)
	}
	if len(tpl.Data) != tpl.Rows*tpl.Cols {
		t.Fatalf("data length mismatch: got %d want %d", len(tpl.Data), tpl.Rows*tpl.Cols)
	}
}

func TestNewChromaTemplateDimensions(t *testing.T) {
	tpl := NewChromaTemplate(0x4567, 0, nil, 7)
	if tpl.Rows != 38 || tpl.Cols != 38 {
		t.Fatalf("chroma template wrong shape: %dx%d", tpl.Rows, tpl.Cols)
	}
}

func TestTemplateSampleWraps(t *testing.T) {
	tpl := Template{Data: []int16{1, 2, 3, 4}, Rows: 2, Cols: 2}
	if tpl.Sample(0, 0) != 1 {
		t.Fatalf("got %d want 1", tpl.Sample(0, 0))
	}
	if tpl.Sample(2, 0) != 1 { // wraps to row 0
		t.Fatalf("row wrap failed: %d", tpl.Sample(2, 0))
	}
	if tpl.Sample(0, -1) != 2 { // wraps to col 1
		t.Fatalf("negative col wrap failed: %d", tpl.Sample(0, -1))
	}
}

func TestApplyWithTemplateModifiesOutput(t *testing.T) {
	plane := make([]uint8, 64*64)
	for i := range plane {
		plane[i] = 128
	}
	var lut ScalingLUT
	for i := range lut {
		lut[i] = 200
	}
	tpl := NewLumaTemplate(0x9999, 0, nil, 7)
	p := &Params{GrainSeed: 0xA5A5, ScalingShift: 8}
	ApplyWithTemplate(plane, 64, 64, 64, &lut, &tpl, p)
	diffs := 0
	for _, v := range plane {
		if v != 128 {
			diffs++
		}
	}
	if diffs == 0 {
		t.Fatal("templated grain produced no changes")
	}
}

func TestApplyWithTemplateZeroSeedNoop(t *testing.T) {
	plane := make([]uint8, 32*32)
	for i := range plane {
		plane[i] = 77
	}
	var lut ScalingLUT
	for i := range lut {
		lut[i] = 255
	}
	tpl := NewLumaTemplate(0x1111, 0, nil, 7)
	p := &Params{GrainSeed: 0, ScalingShift: 8}
	ApplyWithTemplate(plane, 32, 32, 32, &lut, &tpl, p)
	for _, v := range plane {
		if v != 77 {
			t.Fatal("zero-seed modified a pixel")
		}
	}
}

func TestApplyWithTemplateClamps(t *testing.T) {
	plane := make([]uint8, 32*32)
	for i := range plane {
		plane[i] = 250
	}
	var lut ScalingLUT
	for i := range lut {
		lut[i] = 255
	}
	tpl := NewLumaTemplate(0x2222, 0, nil, 7)
	p := &Params{GrainSeed: 0xCAFE, ScalingShift: 8}
	ApplyWithTemplate(plane, 32, 32, 32, &lut, &tpl, p)
	for i, v := range plane {
		if v > 255 {
			t.Fatalf("pixel %d exceeded 255: %d", i, v)
		}
	}
}
