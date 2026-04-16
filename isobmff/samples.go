package isobmff

import "fmt"

// Sample describes one encoded image frame within an AVIF image
// sequence. Offset is absolute within the source file; Size is the
// frame's byte length; Duration is its presentation time delta in
// the movie's timescale (see Mvhd.Timescale); IsSync marks sync
// samples (keyframes that can be decoded standalone). When stss is
// absent every sample is a sync sample.
type Sample struct {
	Offset   uint64
	Size     uint32
	Duration uint32
	IsSync   bool
}

// SampleTable walks the stts / stsc / stsz / stco+co64 trio under an
// Stbl and returns a flat list of per-sample (offset, size, duration)
// records. Only one track's worth of samples is resolved — AVIF
// sequences contain a single image track.
//
// Returns an error when the expected boxes are missing or malformed.
func (s *Stbl) SampleTable() ([]Sample, error) {
	var (
		stts *Stts
		stsc *Stsc
		stsz *Stsz
		stco *Stco
		co64 *Co64
		stss *Stss
	)
	for _, ch := range s.Children {
		switch b := ch.(type) {
		case *Stts:
			stts = b
		case *Stsc:
			stsc = b
		case *Stsz:
			stsz = b
		case *Stco:
			stco = b
		case *Co64:
			co64 = b
		case *Stss:
			stss = b
		}
	}
	if stts == nil {
		return nil, fmt.Errorf("%w: stbl missing stts", ErrInvalid)
	}
	if stsc == nil {
		return nil, fmt.Errorf("%w: stbl missing stsc", ErrInvalid)
	}
	if stsz == nil {
		return nil, fmt.Errorf("%w: stbl missing stsz", ErrInvalid)
	}
	chunkCount := 0
	chunkOffset := func(i int) uint64 {
		if stco != nil {
			return uint64(stco.Offsets[i])
		}
		return co64.Offsets[i]
	}
	switch {
	case stco != nil:
		chunkCount = len(stco.Offsets)
	case co64 != nil:
		chunkCount = len(co64.Offsets)
	default:
		return nil, fmt.Errorf("%w: stbl missing stco/co64", ErrInvalid)
	}

	// Expand stsc into a per-chunk samples-per-chunk slice.
	perChunk := make([]uint32, chunkCount)
	descIdx := make([]uint32, chunkCount)
	for i, e := range stsc.Entries {
		start := int(e.FirstChunk) - 1 // 1-based in the box, 0-based here
		end := chunkCount
		if i+1 < len(stsc.Entries) {
			end = int(stsc.Entries[i+1].FirstChunk) - 1
		}
		if start < 0 || end > chunkCount || start > end {
			return nil, fmt.Errorf("%w: stsc entry %d out of range", ErrInvalid, i)
		}
		for c := start; c < end; c++ {
			perChunk[c] = e.SamplesPerChunk
			descIdx[c] = e.DescriptionIdx
		}
	}

	deltas := stts.SampleDeltas()
	var out []Sample
	sampleIdx := uint32(0) // 0-based
	for c := 0; c < chunkCount; c++ {
		off := chunkOffset(c)
		for i := uint32(0); i < perChunk[c]; i++ {
			size := stsz.SizeOf(sampleIdx + 1)
			var dur uint32
			if int(sampleIdx) < len(deltas) {
				dur = deltas[sampleIdx]
			}
			out = append(out, Sample{
				Offset:   off,
				Size:     size,
				Duration: dur,
				IsSync:   stss.IsSync(sampleIdx + 1),
			})
			off += uint64(size)
			sampleIdx++
		}
	}
	return out, nil
}

// ImageTrackStbl returns the Stbl of the first video/image track in
// the Moov, or nil if not present.
func (m *Moov) ImageTrackStbl() *Stbl {
	for _, ch := range m.Children {
		trak, ok := ch.(*Trak)
		if !ok {
			continue
		}
		for _, tc := range trak.Children {
			mdia, ok := tc.(*Mdia)
			if !ok {
				continue
			}
			for _, mc := range mdia.Children {
				minf, ok := mc.(*Minf)
				if !ok {
					continue
				}
				for _, mic := range minf.Children {
					if stbl, ok := mic.(*Stbl); ok {
						return stbl
					}
				}
			}
		}
	}
	return nil
}
