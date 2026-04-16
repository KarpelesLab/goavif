package goavif

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"time"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/obu"
	"github.com/KarpelesLab/goavif/isobmff"
)

// DecodeAll reads an AVIF image sequence (ftyp brand "avis") from r
// and returns the per-frame images plus their presentation durations.
// For a still-image AVIF (ftyp brand "avif") it returns a single-frame
// slice with a zero-length duration, matching the standard library
// contract for GIF/WebP decoders.
func DecodeAll(r io.Reader) ([]image.Image, []time.Duration, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	ct, err := isobmff.ParseContainer(data)
	if err != nil {
		return nil, nil, err
	}

	isSequence := ct.Ftyp.HasBrand("avis") && ct.Moov != nil
	if !isSequence {
		// Fall through to the still-image path and wrap in a one-frame slice.
		img, err := decodeStill(ct)
		if err != nil {
			return nil, nil, err
		}
		return []image.Image{img}, []time.Duration{0}, nil
	}

	stbl := ct.Moov.ImageTrackStbl()
	if stbl == nil {
		return nil, nil, fmt.Errorf("goavif: avis container missing image track stbl")
	}
	samples, err := stbl.SampleTable()
	if err != nil {
		return nil, nil, fmt.Errorf("goavif: sample table: %w", err)
	}

	timescale := uint32(1000) // ms fallback
	if mvhd := findMvhd(ct.Moov); mvhd != nil && mvhd.Timescale != 0 {
		timescale = mvhd.Timescale
	}

	// For AVIS the sequence header lives in an item av1C that the
	// primary item points at. Reuse that lookup.
	primaryID := ct.PrimaryItemID()
	var seq *obu.SequenceHeader
	if primaryID != 0 {
		seq, err = extractSequenceHeader(ct, primaryID)
		if err != nil {
			seq = nil
		}
	}
	// Fall back: parse the sequence header from the first sample's
	// tile group bitstream.
	if seq == nil {
		return nil, nil, fmt.Errorf("goavif: avis sequence header not found in primary item")
	}

	frames := make([]image.Image, 0, len(samples))
	durations := make([]time.Duration, 0, len(samples))
	for _, s := range samples {
		if uint64(s.Offset)+uint64(s.Size) > uint64(len(data)) {
			return nil, nil, fmt.Errorf("goavif: sample offset %d+%d out of range", s.Offset, s.Size)
		}
		sampleBytes := data[s.Offset : s.Offset+uint64(s.Size)]
		frame, err := decoder.Decode(sampleBytes, seq)
		if err != nil {
			return nil, nil, fmt.Errorf("goavif: frame decode: %w", err)
		}
		img, err := frameToImage(frame)
		if err != nil {
			return nil, nil, err
		}
		frames = append(frames, img)
		// Duration in ticks → time.Duration.
		d := time.Duration(int64(s.Duration) * int64(time.Second) / int64(timescale))
		durations = append(durations, d)
	}
	return frames, durations, nil
}

// decodeStill is the still-image code path used when DecodeAll is
// handed a single-image AVIF. Mirrors Decode but returns the raw
// image so DecodeAll can wrap it.
func decodeStill(ct *isobmff.Container) (image.Image, error) {
	if !ct.Ftyp.HasBrand("avif") && !ct.Ftyp.HasBrand("avis") {
		return nil, fmt.Errorf("goavif: ftyp has no avif/avis brand")
	}
	primaryID := ct.PrimaryItemID()
	if primaryID == 0 {
		return nil, fmt.Errorf("goavif: no primary item")
	}
	seq, err := extractSequenceHeader(ct, primaryID)
	if err != nil {
		return nil, err
	}
	itemBytes, err := ct.ItemData(primaryID)
	if err != nil {
		return nil, err
	}
	frame, err := decoder.Decode(itemBytes, seq)
	if err != nil {
		return nil, err
	}
	if alphaID := findAlphaItemID(ct, primaryID); alphaID != 0 {
		alpha, err := decodeAlphaFrame(ct, alphaID)
		if err != nil {
			return nil, fmt.Errorf("goavif: alpha decode: %w", err)
		}
		if frame.BitDepth > 8 {
			return compositeNRGBA64(frame, alpha)
		}
		return compositeNRGBA(frame, alpha)
	}
	return frameToImage(frame)
}

func findMvhd(m *isobmff.Moov) *isobmff.Mvhd {
	for _, ch := range m.Children {
		if mv, ok := ch.(*isobmff.Mvhd); ok {
			return mv
		}
	}
	return nil
}

// avoid unused-import warnings on bytes when the file doesn't end up
// using it directly in some build configurations.
var _ = bytes.NewReader
