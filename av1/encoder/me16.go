package encoder

import (
	"sync"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/predict"
)

var mePred16Pool = sync.Pool{
	New: func() any {
		b := make([]uint16, 64*64)
		return &b
	},
}

// SearchMV16 is the HBD counterpart of [SearchMV]: full-window SAD
// search over a uint16 reference plane.
func SearchMV16(
	srcY []uint16, srcStride int,
	bx, by, bw, bh int,
	refY []uint16, refW, refH, refStride int,
	searchRange int,
) decoder.MV {
	bestSAD := sadAtClamped16(srcY, srcStride, bx, by, bw, bh, refY, refW, refH, refStride, bx, by)
	bestDX, bestDY := 0, 0
	for dy := -searchRange; dy <= searchRange; dy++ {
		for dx := -searchRange; dx <= searchRange; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			cost := sadAtClamped16(srcY, srcStride, bx, by, bw, bh, refY, refW, refH, refStride, bx+dx, by+dy)
			if cost < bestSAD {
				bestSAD = cost
				bestDX, bestDY = dx, dy
			}
		}
	}
	return decoder.MV{Row: int32(bestDY) * 8, Col: int32(bestDX) * 8}
}

// sadAtClamped16 is the HBD counterpart of sadAtClamped.
func sadAtClamped16(
	src []uint16, srcStride int, sx, sy, bw, bh int,
	ref []uint16, refW, refH, refStride int, rx, ry int,
) int {
	sum := 0
	for r := 0; r < bh; r++ {
		sRow := (sy + r) * srcStride
		ry2 := ry + r
		if ry2 < 0 {
			ry2 = 0
		} else if ry2 >= refH {
			ry2 = refH - 1
		}
		rRow := ry2 * refStride
		for c := 0; c < bw; c++ {
			rx2 := rx + c
			if rx2 < 0 {
				rx2 = 0
			} else if rx2 >= refW {
				rx2 = refW - 1
			}
			d := int(src[sRow+sx+c]) - int(ref[rRow+rx2])
			if d < 0 {
				d = -d
			}
			sum += d
		}
	}
	return sum
}

// DiamondSearchMV16 is the HBD counterpart of [DiamondSearchMV].
func DiamondSearchMV16(
	srcY []uint16, srcStride int,
	bx, by, bw, bh int,
	refY []uint16, refW, refH, refStride int,
	maxSteps int,
) decoder.MV {
	cx, cy := 0, 0
	bestSAD := sadAtClamped16(srcY, srcStride, bx, by, bw, bh, refY, refW, refH, refStride, bx, by)

	offs := [8][2]int{
		{1, 0}, {-1, 0}, {0, 1}, {0, -1},
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
	}
	for step := 0; step < maxSteps; step++ {
		bestDX, bestDY := cx, cy
		improved := false
		for _, o := range offs {
			dx := cx + o[0]
			dy := cy + o[1]
			cost := sadAtClamped16(srcY, srcStride, bx, by, bw, bh, refY, refW, refH, refStride, bx+dx, by+dy)
			if cost < bestSAD {
				bestSAD = cost
				bestDX, bestDY = dx, dy
				improved = true
			}
		}
		if !improved {
			break
		}
		cx, cy = bestDX, bestDY
	}
	return decoder.MV{Row: int32(cy) * 8, Col: int32(cx) * 8}
}

// SubPelRefineMV16 is the HBD counterpart of [SubPelRefineMV].
func SubPelRefineMV16(
	srcY []uint16, srcStride int,
	bx, by, bw, bh int,
	refY []uint16, refW, refH, refStride int,
	integerMV decoder.MV, bitDepth int,
) decoder.MV {
	bestMV := integerMV
	bestSAD := sadForMV16(srcY, srcStride, bx, by, bw, bh,
		refY, refW, refH, refStride, integerMV, bitDepth)

	offs := [8][2]int32{
		{4, 0}, {-4, 0}, {0, 4}, {0, -4},
		{4, 4}, {4, -4}, {-4, 4}, {-4, -4},
	}
	for _, o := range offs {
		mv := decoder.MV{Row: integerMV.Row + o[1], Col: integerMV.Col + o[0]}
		cost := sadForMV16(srcY, srcStride, bx, by, bw, bh,
			refY, refW, refH, refStride, mv, bitDepth)
		if cost < bestSAD {
			bestSAD = cost
			bestMV = mv
		}
	}
	baseMV := bestMV
	offsQ := [8][2]int32{
		{2, 0}, {-2, 0}, {0, 2}, {0, -2},
		{2, 2}, {2, -2}, {-2, 2}, {-2, -2},
	}
	for _, o := range offsQ {
		mv := decoder.MV{Row: baseMV.Row + o[1], Col: baseMV.Col + o[0]}
		cost := sadForMV16(srcY, srcStride, bx, by, bw, bh,
			refY, refW, refH, refStride, mv, bitDepth)
		if cost < bestSAD {
			bestSAD = cost
			bestMV = mv
		}
	}
	return bestMV
}

// sadForMV16 is the HBD counterpart of sadForMV.
func sadForMV16(
	srcY []uint16, srcStride int,
	bx, by, bw, bh int,
	refY []uint16, refW, refH, refStride int,
	mv decoder.MV, bitDepth int,
) int {
	predPtr := mePred16Pool.Get().(*[]uint16)
	pred := *predPtr
	need := bw * bh
	if cap(pred) < need {
		pred = make([]uint16, need)
	} else {
		pred = pred[:need]
	}
	defer func() {
		*predPtr = pred
		mePred16Pool.Put(predPtr)
	}()
	decoder.MotionCompensate16(pred, bw, bh, refY, refW, refH, refStride,
		bx, by, mv, predict.InterpRegular, bitDepth)
	sum := 0
	for r := 0; r < bh; r++ {
		sRow := (by + r) * srcStride
		pRow := r * bw
		for c := 0; c < bw; c++ {
			d := int(srcY[sRow+bx+c]) - int(pred[pRow+c])
			if d < 0 {
				d = -d
			}
			sum += d
		}
	}
	return sum
}
