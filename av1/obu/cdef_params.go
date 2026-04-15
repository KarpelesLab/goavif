package obu

import "github.com/KarpelesLab/goavif/av1/bitio"

// CdefParams decodes cdef_params() (spec §5.9.19).
type CdefParams struct {
	CdefBits          uint8
	YPriStrengths     [8]uint8
	YSecStrengths     [8]uint8
	UVPriStrengths    [8]uint8
	UVSecStrengths    [8]uint8
	CdefDampingMinus3 uint8
}

func parseCdefParams(r *bitio.Reader, c *CdefParams, sh *SequenceHeader, fh *FrameHeader) {
	if fh.CodedLosslessHint() || fh.AllowIntrabc || !sh.EnableCdef {
		c.CdefBits = 0
		c.YPriStrengths[0] = 0
		c.YSecStrengths[0] = 0
		c.UVPriStrengths[0] = 0
		c.UVSecStrengths[0] = 0
		c.CdefDampingMinus3 = 0
		return
	}
	c.CdefDampingMinus3 = uint8(r.F(2))
	c.CdefBits = uint8(r.F(2))
	n := uint8(1) << c.CdefBits
	for i := uint8(0); i < n; i++ {
		c.YPriStrengths[i] = uint8(r.F(4))
		c.YSecStrengths[i] = uint8(r.F(2))
		if c.YSecStrengths[i] == 3 {
			c.YSecStrengths[i]++
		}
		if sh.Color.NumPlanes == 3 {
			c.UVPriStrengths[i] = uint8(r.F(4))
			c.UVSecStrengths[i] = uint8(r.F(2))
			if c.UVSecStrengths[i] == 3 {
				c.UVSecStrengths[i]++
			}
		}
	}
}
