package encoder

import (
	"sync"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/predict"
)

// mePredPool reuses the MC prediction buffer used by sadForMV during
// sub-pel search — each search tests 17+ candidate MVs per block, so
// keeping the allocation off the hot path is worth a pool.
var mePredPool = sync.Pool{
	New: func() any {
		b := make([]uint8, 64*64)
		return &b
	},
}

// SearchMV finds a good integer-pel motion vector for the bw×bh
// block whose top-left is at (bx, by) in the source plane, scanning
// the reference plane within [-searchRange, +searchRange] pixels of
// (0, 0). The returned MV is in eighth-pel units (integer-pel × 8).
//
// The cost is pure SAD; no λ·rate term — callers that want rate-aware
// RDO can wrap this. Ties go to the zero-MV candidate to favor
// skip-worthy blocks.
//
// The reference is sampled with edge clamping (matching the decoder's
// MotionCompensate), so MVs that point past the frame are legal. This
// mirrors the MC path so the ME cost reflects what the decoder will
// actually reconstruct.
func SearchMV(
	srcY []uint8, srcStride int,
	bx, by, bw, bh int,
	refY []uint8, refW, refH, refStride int,
	searchRange int,
) decoder.MV {
	bestSAD := sadAtClamped(srcY, srcStride, bx, by, bw, bh, refY, refW, refH, refStride, bx, by)
	bestDX, bestDY := 0, 0

	for dy := -searchRange; dy <= searchRange; dy++ {
		for dx := -searchRange; dx <= searchRange; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			cost := sadAtClamped(srcY, srcStride, bx, by, bw, bh, refY, refW, refH, refStride, bx+dx, by+dy)
			if cost < bestSAD {
				bestSAD = cost
				bestDX, bestDY = dx, dy
			}
		}
	}
	return decoder.MV{Row: int32(bestDY) * 8, Col: int32(bestDX) * 8}
}

// sadAtClamped computes sum-of-absolute-differences between a bw×bh
// block of src starting at (sx, sy) and a same-sized region of ref
// starting at (rx, ry), with ref coordinates clamped to the reference
// bounds. This mirrors the decoder's MotionCompensate edge-clamp so
// the ME cost reflects what the decoder will actually reconstruct.
func sadAtClamped(
	src []uint8, srcStride int, sx, sy, bw, bh int,
	ref []uint8, refW, refH, refStride int, rx, ry int,
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

// SubPelRefineMV refines an integer-pel MV to eighth-pel precision by
// sampling 3 phases per axis ({-4, 0, +4}, i.e. half-pel steps) around
// the starting MV, picking the best via SAD against the reference's
// motion-compensated prediction.
//
// Returns an MV with the refined sub-pel phase in eighth-pel units.
// The integer-pel component of the result equals the input; only the
// low 3 bits (phase) change. Call this after SearchMV or
// DiamondSearchMV to convert coarse integer-pel matches into sub-pel
// matches.
func SubPelRefineMV(
	srcY []uint8, srcStride int,
	bx, by, bw, bh int,
	refY []uint8, refW, refH, refStride int,
	integerMV decoder.MV,
) decoder.MV {
	bestMV := integerMV
	bestSAD := sadForMV(srcY, srcStride, bx, by, bw, bh,
		refY, refW, refH, refStride, integerMV)

	// Half-pel refinement grid: 8 neighbors at phase ±4.
	offs := [8][2]int32{
		{4, 0}, {-4, 0}, {0, 4}, {0, -4},
		{4, 4}, {4, -4}, {-4, 4}, {-4, -4},
	}
	for _, o := range offs {
		mv := decoder.MV{Row: integerMV.Row + o[1], Col: integerMV.Col + o[0]}
		cost := sadForMV(srcY, srcStride, bx, by, bw, bh,
			refY, refW, refH, refStride, mv)
		if cost < bestSAD {
			bestSAD = cost
			bestMV = mv
		}
	}

	// Quarter-pel refinement around the best half-pel match.
	baseMV := bestMV
	offsQ := [8][2]int32{
		{2, 0}, {-2, 0}, {0, 2}, {0, -2},
		{2, 2}, {2, -2}, {-2, 2}, {-2, -2},
	}
	for _, o := range offsQ {
		mv := decoder.MV{Row: baseMV.Row + o[1], Col: baseMV.Col + o[0]}
		cost := sadForMV(srcY, srcStride, bx, by, bw, bh,
			refY, refW, refH, refStride, mv)
		if cost < bestSAD {
			bestSAD = cost
			bestMV = mv
		}
	}
	return bestMV
}

// sadForMV computes SAD between src block and the motion-compensated
// reference prediction at the given MV. Handles both integer-pel
// (direct copy) and sub-pel (8-tap filter) paths.
func sadForMV(
	srcY []uint8, srcStride int,
	bx, by, bw, bh int,
	refY []uint8, refW, refH, refStride int,
	mv decoder.MV,
) int {
	predPtr := mePredPool.Get().(*[]uint8)
	pred := *predPtr
	need := bw * bh
	if cap(pred) < need {
		pred = make([]uint8, need)
	} else {
		pred = pred[:need]
	}
	defer func() {
		*predPtr = pred
		mePredPool.Put(predPtr)
	}()
	decoder.MotionCompensate(pred, bw, bh, refY, refW, refH, refStride,
		bx, by, mv, predict.InterpRegular)
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

// DiamondSearchMV is a faster alternative to SearchMV: starts at the
// zero MV, evaluates the 4 diamond neighbors, moves to the best, and
// repeats until no neighbor improves the SAD. Trades a small amount of
// search quality for ~N× fewer SAD evaluations on large windows.
//
// The final MV is returned in eighth-pel units (integer-pel × 8).
func DiamondSearchMV(
	srcY []uint8, srcStride int,
	bx, by, bw, bh int,
	refY []uint8, refW, refH, refStride int,
	maxSteps int,
) decoder.MV {
	cx, cy := 0, 0
	bestSAD := sadAtClamped(srcY, srcStride, bx, by, bw, bh, refY, refW, refH, refStride, bx, by)

	// Eight-point diamond — cardinals + diagonals. Step 1 pel at a
	// time so we stay integer-pel.
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
			cost := sadAtClamped(srcY, srcStride, bx, by, bw, bh, refY, refW, refH, refStride, bx+dx, by+dy)
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
