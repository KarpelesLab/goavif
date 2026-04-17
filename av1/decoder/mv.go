package decoder

import (
	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
)

// MVJoint enumerates the four mv_joint values (spec §6.10.27).
type MVJoint uint8

const (
	MVJointZero   MVJoint = 0 // both components zero
	MVJointHNZVZ  MVJoint = 1 // horizontal non-zero, vertical zero
	MVJointHZVNZ  MVJoint = 2 // horizontal zero, vertical non-zero
	MVJointHNZVNZ MVJoint = 3 // both non-zero
)

// MV is a single motion vector (quarter-pel precision by default;
// eighth-pel when allow_high_precision_mv is set — AVIF stills
// always use integer-pel so we keep it simple).
type MV struct {
	Row int32
	Col int32
}

// MVDecoder reads motion vectors from the entropy stream. It holds
// mutable CDFs so CDF adaptation (when enabled) can track symbol
// frequencies across the frame.
type MVDecoder struct {
	dec             *entropy.Decoder
	jointCDF        cdfs.CDF
	signCDF         [2]cdfs.CDF
	classCDF        [2]cdfs.CDF
	class0BitCDF    [2]cdfs.CDF
	class0FrCDF     [2][2]cdfs.CDF
	class0HpCDF     [2]cdfs.CDF
	frCDF           [2]cdfs.CDF
	hpCDF           [2]cdfs.CDF
	bitsCDF         [2][10]cdfs.CDF
	allowHighPrecMV bool
}

// InitMVDecoder returns a fresh MVDecoder primed with default CDFs.
// allowHighPrecMV turns on the 1/8-pel path; AVIF still images
// force integer-pel, so pass false.
func InitMVDecoder(dec *entropy.Decoder, allowHighPrecMV bool) *MVDecoder {
	m := &MVDecoder{dec: dec, allowHighPrecMV: allowHighPrecMV}
	m.jointCDF = append(cdfs.CDF(nil), cdfs.DefaultMvJointCDF...)
	for c := 0; c < 2; c++ {
		m.signCDF[c] = append(cdfs.CDF(nil), cdfs.DefaultMvSignCDF[c]...)
		m.classCDF[c] = append(cdfs.CDF(nil), cdfs.DefaultMvClassCDF[c]...)
		m.class0BitCDF[c] = append(cdfs.CDF(nil), cdfs.DefaultMvClass0BitCDF[c]...)
		m.class0HpCDF[c] = append(cdfs.CDF(nil), cdfs.DefaultMvClass0HpCDF[c]...)
		m.frCDF[c] = append(cdfs.CDF(nil), cdfs.DefaultMvFrCDF[c]...)
		m.hpCDF[c] = append(cdfs.CDF(nil), cdfs.DefaultMvHpCDF[c]...)
		for b := 0; b < 2; b++ {
			m.class0FrCDF[c][b] = append(cdfs.CDF(nil), cdfs.DefaultMvClass0FrCDF[c][b]...)
		}
		for i := 0; i < 10; i++ {
			m.bitsCDF[c][i] = append(cdfs.CDF(nil), cdfs.DefaultMvBitsCDF[c][i]...)
		}
	}
	return m
}

// ReadMV decodes a motion vector difference. The MV is added to the
// predicted MV by the caller; this function returns only the diff
// component.
func (m *MVDecoder) ReadMV() MV {
	j := MVJoint(m.dec.DecodeSymbol(m.jointCDF))
	var mv MV
	if j == MVJointHNZVZ || j == MVJointHNZVNZ {
		mv.Col = m.readComponent(0)
	}
	if j == MVJointHZVNZ || j == MVJointHNZVNZ {
		mv.Row = m.readComponent(1)
	}
	return mv
}

// readComponent decodes a single MV component (horizontal if
// comp==0, vertical if comp==1). Result is in quarter-pel units
// (or 1/8 when allowHighPrecMV and relevant class0 / fr / hp bits
// are coded).
func (m *MVDecoder) readComponent(comp int) int32 {
	sign := m.dec.DecodeSymbol(m.signCDF[comp])
	cls := m.dec.DecodeSymbol(m.classCDF[comp])

	// Class 0: magnitude 0..3 in integer pel, plus fractional bits.
	var magInt int32
	var frac int32
	var hp int32
	if cls == 0 {
		b := int32(m.dec.DecodeSymbol(m.class0BitCDF[comp]))
		magInt = b
		frac = int32(m.dec.DecodeSymbol(m.class0FrCDF[comp][b]))
		if m.allowHighPrecMV {
			hp = int32(m.dec.DecodeSymbol(m.class0HpCDF[comp]))
		} else {
			hp = 1
		}
	} else {
		// Class ≥ 1: magInt = CLASS0_SIZE + (bits << class), fed by
		// per-bit binary CDFs.
		bits := int32(0)
		for i := 0; i < cls; i++ {
			b := int32(m.dec.DecodeSymbol(m.bitsCDF[comp][i]))
			bits |= b << uint(i)
		}
		magInt = int32(1<<uint(cls+2)) + (bits << 3)
		frac = int32(m.dec.DecodeSymbol(m.frCDF[comp]))
		if m.allowHighPrecMV {
			hp = int32(m.dec.DecodeSymbol(m.hpCDF[comp]))
		} else {
			hp = 1
		}
	}
	// Combine into full eighth-pel magnitude:
	//   total = magInt*8 (for class≥1, magInt already in eighth-pel)
	//         + frac*2 + hp
	// Class 0 magInt is 0 or 1 (integer pels), so shift.
	var mag int32
	if cls == 0 {
		mag = magInt*8 + frac*2 + hp + 1
	} else {
		mag = magInt + frac*2 + hp + 1
	}
	if sign == 1 {
		return -mag
	}
	return mag
}
