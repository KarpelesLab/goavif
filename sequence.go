package goavif

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"time"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/encoder"
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
	sawNonSync := false
	var prevFrame *decoder.Frame
	for i, s := range samples {
		if uint64(s.Offset)+uint64(s.Size) > uint64(len(data)) {
			return nil, nil, fmt.Errorf("goavif: sample offset %d+%d out of range", s.Offset, s.Size)
		}
		// The decoder only implements intra-only frames today. Non-
		// sync samples require inter prediction (Phase 5+); until that
		// lands, repeat the previous frame for non-sync and flag the
		// situation so callers can detect degraded output.
		if !s.IsSync && i > 0 {
			sawNonSync = true
			frames = append(frames, frames[len(frames)-1])
			durations = append(durations, time.Duration(int64(s.Duration)*int64(time.Second)/int64(timescale)))
			continue
		}
		sampleBytes := data[s.Offset : s.Offset+uint64(s.Size)]
		frame, err := decoder.DecodeWithRef(sampleBytes, seq, prevFrame)
		if err != nil {
			// The decoder may legitimately reject a sample flagged as
			// sync if its internal frame header actually carries an
			// inter frame (unusual but valid). Fall back to repeating
			// the previous frame just like the non-sync branch.
			if errors.Is(err, decoder.ErrInterFrameUnsupported) && i > 0 {
				sawNonSync = true
				frames = append(frames, frames[len(frames)-1])
				durations = append(durations, time.Duration(int64(s.Duration)*int64(time.Second)/int64(timescale)))
				continue
			}
			return nil, nil, fmt.Errorf("goavif: frame %d decode: %w", i, err)
		}
		prevFrame = frame
		img, err := frameToImage(frame)
		if err != nil {
			return nil, nil, err
		}
		frames = append(frames, img)
		d := time.Duration(int64(s.Duration) * int64(time.Second) / int64(timescale))
		durations = append(durations, d)
	}
	if sawNonSync {
		return frames, durations, ErrInterPredictionNotImplemented
	}
	return frames, durations, nil
}

// ErrInterPredictionNotImplemented is returned by DecodeAll when the
// sample table marks a frame as a non-sync (inter-predicted) frame.
// The intra-only decoder fills those slots by repeating the last
// decoded frame. Callers can check with errors.Is and decide whether
// the degraded output is acceptable.
var ErrInterPredictionNotImplemented = fmt.Errorf("goavif: inter-predicted frames not yet implemented (Phase 5)")

// EncodeAll writes a sequence of images to w as an AVIS image
// sequence. Each frame is coded as a self-contained intra-only AV1
// keyframe — there is no inter prediction, so random access is
// perfect but compression is lower than a true video codec.
//
// delays carries per-frame presentation durations. Frames without a
// matching delay (shorter slice) inherit 100ms. All images must
// share dimensions and bit depth; mismatched frames return an error.
func EncodeAll(w io.Writer, frames []image.Image, delays []time.Duration, opts *Options) error {
	if len(frames) == 0 {
		return fmt.Errorf("goavif: EncodeAll: no frames")
	}
	ref := frames[0]
	if ref == nil {
		return fmt.Errorf("goavif: EncodeAll: frame 0 is nil")
	}
	bounds := ref.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width < 4 || height < 4 {
		return fmt.Errorf("goavif: EncodeAll: frame too small (%dx%d)", width, height)
	}
	for i, fr := range frames {
		if fr == nil {
			return fmt.Errorf("goavif: EncodeAll: frame %d is nil", i)
		}
		fb := fr.Bounds()
		if fb.Dx() != width || fb.Dy() != height {
			return fmt.Errorf("goavif: EncodeAll: frame %d size %dx%d differs from %dx%d",
				i, fb.Dx(), fb.Dy(), width, height)
		}
	}

	baseQ := uint8(32)
	if opts != nil && opts.Quality > 0 && opts.Quality <= 100 {
		baseQ = uint8(255 - (opts.Quality*255)/100)
	}

	monochrome := isGrayscale(ref)
	bitDepth := hbdBitDepth(ref, opts)
	hbd := bitDepth > 8

	// Build the shared sequence header once. Every frame uses it,
	// which means the av1C's ConfigOBUs can be the same across the
	// whole sequence.
	var seqPayload []byte
	switch {
	case monochrome && hbd:
		seqPayload = obu.WriteMonoSequenceHeaderHBD(width, height, bitDepth)
	case monochrome:
		seqPayload = obu.WriteMonoSequenceHeader(width, height)
	case hbd:
		seqPayload = obu.WriteSequenceHeaderHBD(width, height, bitDepth)
	default:
		seqPayload = obu.WriteSequenceHeader(width, height)
	}
	sh, err := obu.ParseSequenceHeader(seqPayload)
	if err != nil {
		return err
	}

	var framePayload []byte
	if monochrome {
		framePayload = obu.WriteMonoKeyFrameHeader(width, height, baseQ)
	} else {
		framePayload = obu.WriteKeyFrameHeader(width, height, baseQ)
	}
	fh, _, err := obu.ParseFrameHeaderBytes(framePayload, sh, nil)
	if err != nil {
		return err
	}

	seqOBU := obu.WrapOBU(1, seqPayload)

	// Encode each frame.
	const timescale = uint32(1000) // ms
	seqFrames := make([]isobmff.SequenceFrame, 0, len(frames))
	for i, fr := range frames {
		tilePayload, err := encodeFrameTile(width, height, fh, sh, fr, bitDepth, hbd, monochrome)
		if err != nil {
			return fmt.Errorf("goavif: EncodeAll: frame %d: %w", i, err)
		}
		frameBytes := append(append([]byte(nil), framePayload...), tilePayload...)
		frameOBU := obu.WrapOBU(6, frameBytes)
		// Each sample is self-contained: include the seq OBU so any
		// sample can be decoded standalone (matches the still-image
		// path's item shape).
		sampleBytes := append(append([]byte(nil), seqOBU...), frameOBU...)

		var delay time.Duration = 100 * time.Millisecond
		if i < len(delays) {
			delay = delays[i]
		}
		ticks := uint32(int64(delay) * int64(timescale) / int64(time.Second))
		if ticks == 0 {
			ticks = 1
		}
		seqFrames = append(seqFrames, isobmff.SequenceFrame{
			AV1Bitstream:  sampleBytes,
			DurationTicks: ticks,
		})
	}

	container, err := isobmff.BuildSequence(isobmff.Sequence{
		Width:              uint32(width),
		Height:             uint32(height),
		BitDepth:           sh.Color.BitDepth,
		Monochrome:         sh.Color.Monochrome,
		ChromaSubsamplingX: sh.Color.SubsamplingX,
		ChromaSubsamplingY: sh.Color.SubsamplingY,
		ConfigOBUs:         seqOBU,
		Timescale:          timescale,
		Frames:             seqFrames,
	})
	if err != nil {
		return fmt.Errorf("goavif: EncodeAll: build container: %w", err)
	}
	_, err = w.Write(container)
	return err
}

// encodeFrameTile produces the tile-group payload for a single frame,
// honoring the resolved (bitDepth, monochrome, hbd) flags.
func encodeFrameTile(width, height int, fh *obu.FrameHeader, sh *obu.SequenceHeader,
	m image.Image, bitDepth int, hbd, monochrome bool) ([]byte, error) {
	if hbd {
		var y16, u16, v16 []uint16
		if monochrome {
			y16 = imageToLuma16(m, bitDepth)
		} else {
			y16, u16, v16 = imageToYUV420_16(m, bitDepth)
		}
		return encoder.WriteIntraOnlyTile16(width, height, fh, sh, y16, u16, v16)
	}
	var y, u, v []uint8
	if monochrome {
		y = imageToLuma(m)
	} else {
		y, u, v = imageToYUV420(m)
	}
	return encoder.WriteIntraOnlyTile(width, height, fh, sh, y, u, v)
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
