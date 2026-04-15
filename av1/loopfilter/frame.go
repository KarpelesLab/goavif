package loopfilter

// Plane carries the bytes of a single image plane, plus its row stride.
type Plane struct {
	Pix    []uint8
	Stride int
	Width  int
	Height int
}

// EdgeGrid describes the position of internal block edges that the
// deblocking pass should consider. EdgeXs holds the x-coordinates of
// vertical edges (excluding 0 and Plane.Width); EdgeYs holds the
// y-coordinates of horizontal edges. Both slices must be sorted ascending.
//
// This is a simplification — the AV1 deblocking pass actually walks every
// 4-sample edge and only filters where transform-block boundaries fall.
// For intra-only stills with a uniform TX size the simpler grid form
// suffices; the full per-edge driver lands with the bitstream-driven tile
// decoder.
type EdgeGrid struct {
	EdgeXs []int
	EdgeYs []int
}

// ApplyFrameNarrow applies the 4-tap narrow filter to every internal edge
// in the given grid. The plane is mutated in place.
//
// Edges within 2 samples of the plane border are skipped because Filter4
// reads p1 / q1 outside the immediate edge.
func ApplyFrameNarrow(p Plane, grid EdgeGrid, th Thresholds) {
	for _, x := range grid.EdgeXs {
		if x < 2 || x > p.Width-2 {
			continue
		}
		ApplyVerticalEdge4(p.Pix, p.Stride, x, p.Height, th)
	}
	for _, y := range grid.EdgeYs {
		if y < 2 || y > p.Height-2 {
			continue
		}
		ApplyHorizontalEdge4(p.Pix, p.Stride, y, p.Width, th)
	}
}

// UniformGrid returns an EdgeGrid for a plane of size w×h whose internal
// block grid has cells of size cellW × cellH. Used by the simplest tile
// decoders before the full per-block transform-grid tracking lands.
func UniformGrid(w, h, cellW, cellH int) EdgeGrid {
	var g EdgeGrid
	for x := cellW; x < w; x += cellW {
		g.EdgeXs = append(g.EdgeXs, x)
	}
	for y := cellH; y < h; y += cellH {
		g.EdgeYs = append(g.EdgeYs, y)
	}
	return g
}
