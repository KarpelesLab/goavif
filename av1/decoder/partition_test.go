package decoder

import "testing"

func TestWalkPartitionNone(t *testing.T) {
	oracle := func(x, y int, bs BlockSize) PartitionType { return PartitionNone }
	var got []SubBlock
	WalkPartition(0, 0, Block64x64, oracle, func(b SubBlock) { got = append(got, b) })
	if len(got) != 1 || got[0].Size != Block64x64 {
		t.Fatalf("got %+v, want one 64x64 block", got)
	}
}

func TestWalkPartitionSplitOnce(t *testing.T) {
	// Split once at 64x64, NONE at 32x32.
	oracle := func(x, y int, bs BlockSize) PartitionType {
		if bs == Block64x64 {
			return PartitionSplit
		}
		return PartitionNone
	}
	var got []SubBlock
	WalkPartition(0, 0, Block64x64, oracle, func(b SubBlock) { got = append(got, b) })
	if len(got) != 4 {
		t.Fatalf("want 4 leaves, got %d", len(got))
	}
	want := [][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}}
	for i, w := range want {
		if got[i].X != w[0] || got[i].Y != w[1] || got[i].Size != Block32x32 {
			t.Errorf("leaf %d: got (%d,%d,%v) want (%d,%d,Block32x32)", i, got[i].X, got[i].Y, got[i].Size, w[0], w[1])
		}
	}
}

func TestWalkPartitionFullSplitToMin(t *testing.T) {
	// Always split — walks all the way down to 4x4.
	oracle := func(x, y int, bs BlockSize) PartitionType { return PartitionSplit }
	var got []SubBlock
	WalkPartition(0, 0, Block16x16, oracle, func(b SubBlock) { got = append(got, b) })
	// A 16x16 fully split = 16 4x4 blocks.
	if len(got) != 16 {
		t.Fatalf("want 16 leaves, got %d", len(got))
	}
	for _, b := range got {
		if b.Size != Block4x4 {
			t.Errorf("leaf size %v != Block4x4", b.Size)
		}
	}
}

func TestWalkPartitionHorz(t *testing.T) {
	oracle := func(x, y int, bs BlockSize) PartitionType {
		if bs == Block32x32 {
			return PartitionHorz
		}
		return PartitionNone
	}
	var got []SubBlock
	WalkPartition(0, 0, Block32x32, oracle, func(b SubBlock) { got = append(got, b) })
	if len(got) != 2 {
		t.Fatalf("want 2 leaves, got %d", len(got))
	}
	if got[0].Size != Block32x16 || got[1].Size != Block32x16 {
		t.Errorf("non-32x16 leaves: %v, %v", got[0].Size, got[1].Size)
	}
	if got[0].Y != 0 || got[1].Y != 16 {
		t.Errorf("horz halves at wrong Y: %d, %d", got[0].Y, got[1].Y)
	}
}
