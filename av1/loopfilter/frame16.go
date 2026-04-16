package loopfilter

// Plane16 is the uint16 counterpart of [Plane].
type Plane16 struct {
	Pix    []uint16
	Stride int
	Width  int
	Height int
}

// ApplyFrameNarrow16 is the uint16 counterpart of [ApplyFrameNarrow].
func ApplyFrameNarrow16(p Plane16, grid EdgeGrid, th Thresholds16) {
	for _, x := range grid.EdgeXs {
		if x < 2 || x > p.Width-2 {
			continue
		}
		ApplyVerticalEdge4_16(p.Pix, p.Stride, x, p.Height, th)
	}
	for _, y := range grid.EdgeYs {
		if y < 2 || y > p.Height-2 {
			continue
		}
		ApplyHorizontalEdge4_16(p.Pix, p.Stride, y, p.Width, th)
	}
}
