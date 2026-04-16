package isobmff

import (
	"errors"
	"testing"
)

func TestSampleTableExpandsSttsStscStszStco(t *testing.T) {
	// 3 samples, all in one chunk at offset 1000. Sizes 100 / 200 / 300.
	// Deltas 16 each (timescale ticks).
	stts := &Stts{Entries: []SttsEntry{{Count: 3, Delta: 16}}}
	stsc := &Stsc{Entries: []StscEntry{{FirstChunk: 1, SamplesPerChunk: 3, DescriptionIdx: 1}}}
	stsz := &Stsz{SampleSize: 0, SampleCount: 3, Sizes: []uint32{100, 200, 300}}
	stco := &Stco{Offsets: []uint32{1000}}
	stbl := &Stbl{Children: []Box{stts, stsc, stsz, stco}}
	samples, err := stbl.SampleTable()
	if err != nil {
		t.Fatalf("SampleTable: %v", err)
	}
	if len(samples) != 3 {
		t.Fatalf("got %d samples, want 3", len(samples))
	}
	want := []Sample{
		{Offset: 1000, Size: 100, Duration: 16, IsSync: true},
		{Offset: 1100, Size: 200, Duration: 16, IsSync: true},
		{Offset: 1300, Size: 300, Duration: 16, IsSync: true},
	}
	for i, s := range samples {
		if s != want[i] {
			t.Fatalf("sample %d = %+v, want %+v", i, s, want[i])
		}
	}
}

func TestSampleTableMultipleChunks(t *testing.T) {
	// 4 samples spread across 2 chunks (2 each). Uniform sizes.
	stts := &Stts{Entries: []SttsEntry{{Count: 4, Delta: 8}}}
	stsc := &Stsc{Entries: []StscEntry{{FirstChunk: 1, SamplesPerChunk: 2, DescriptionIdx: 1}}}
	stsz := &Stsz{SampleSize: 50, SampleCount: 4}
	stco := &Stco{Offsets: []uint32{100, 500}}
	stbl := &Stbl{Children: []Box{stts, stsc, stsz, stco}}
	samples, err := stbl.SampleTable()
	if err != nil {
		t.Fatalf("SampleTable: %v", err)
	}
	want := []Sample{
		{Offset: 100, Size: 50, Duration: 8, IsSync: true},
		{Offset: 150, Size: 50, Duration: 8, IsSync: true},
		{Offset: 500, Size: 50, Duration: 8, IsSync: true},
		{Offset: 550, Size: 50, Duration: 8, IsSync: true},
	}
	for i, s := range samples {
		if s != want[i] {
			t.Fatalf("sample %d = %+v, want %+v", i, s, want[i])
		}
	}
}

func TestSampleTableMissingBoxesReturnsError(t *testing.T) {
	stbl := &Stbl{} // no children
	_, err := stbl.SampleTable()
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestSampleTableAbsentStssMarksEveryFrameSync(t *testing.T) {
	stts := &Stts{Entries: []SttsEntry{{Count: 3, Delta: 10}}}
	stsc := &Stsc{Entries: []StscEntry{{FirstChunk: 1, SamplesPerChunk: 3, DescriptionIdx: 1}}}
	stsz := &Stsz{SampleSize: 50, SampleCount: 3}
	stco := &Stco{Offsets: []uint32{1000}}
	stbl := &Stbl{Children: []Box{stts, stsc, stsz, stco}}
	samples, err := stbl.SampleTable()
	if err != nil {
		t.Fatalf("SampleTable: %v", err)
	}
	for i, s := range samples {
		if !s.IsSync {
			t.Fatalf("sample %d not marked sync without stss", i)
		}
	}
}

func TestSampleTableStssPicksSubsetOfSyncSamples(t *testing.T) {
	// 4 samples. Only samples 1 and 3 are sync.
	stts := &Stts{Entries: []SttsEntry{{Count: 4, Delta: 10}}}
	stsc := &Stsc{Entries: []StscEntry{{FirstChunk: 1, SamplesPerChunk: 4, DescriptionIdx: 1}}}
	stsz := &Stsz{SampleSize: 100, SampleCount: 4}
	stco := &Stco{Offsets: []uint32{500}}
	stss := &Stss{SampleNumbers: []uint32{1, 3}}
	stbl := &Stbl{Children: []Box{stts, stsc, stsz, stco, stss}}
	samples, err := stbl.SampleTable()
	if err != nil {
		t.Fatalf("SampleTable: %v", err)
	}
	wantSync := []bool{true, false, true, false}
	for i, s := range samples {
		if s.IsSync != wantSync[i] {
			t.Fatalf("sample %d IsSync = %v want %v", i, s.IsSync, wantSync[i])
		}
	}
}

func TestStssIsSyncNilReceiver(t *testing.T) {
	var s *Stss
	for i := uint32(1); i <= 10; i++ {
		if !s.IsSync(i) {
			t.Fatalf("nil stss should report every sample as sync, %d reported false", i)
		}
	}
}

func TestSampleTableWithCo64(t *testing.T) {
	stts := &Stts{Entries: []SttsEntry{{Count: 1, Delta: 32}}}
	stsc := &Stsc{Entries: []StscEntry{{FirstChunk: 1, SamplesPerChunk: 1, DescriptionIdx: 1}}}
	stsz := &Stsz{SampleSize: 999, SampleCount: 1}
	co64 := &Co64{Offsets: []uint64{1 << 34}} // beyond 32-bit range
	stbl := &Stbl{Children: []Box{stts, stsc, stsz, co64}}
	samples, err := stbl.SampleTable()
	if err != nil {
		t.Fatalf("SampleTable: %v", err)
	}
	if samples[0].Offset != 1<<34 || samples[0].Size != 999 || samples[0].Duration != 32 || !samples[0].IsSync {
		t.Fatalf("co64 sample wrong: %+v", samples[0])
	}
}
