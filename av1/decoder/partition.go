package decoder

// BlockSize identifies one of AV1's block shapes (spec §3 BLOCK_*).
type BlockSize uint8

const (
	Block4x4 BlockSize = iota
	Block4x8
	Block8x4
	Block8x8
	Block8x16
	Block16x8
	Block16x16
	Block16x32
	Block32x16
	Block32x32
	Block32x64
	Block64x32
	Block64x64
	Block64x128
	Block128x64
	Block128x128
	Block4x16
	Block16x4
	Block8x32
	Block32x8
	Block16x64
	Block64x16
	BlockInvalid
)

// Width returns the block width in luma samples.
func (b BlockSize) Width() int { return blockWidths[b] }

// Height returns the block height in luma samples.
func (b BlockSize) Height() int { return blockHeights[b] }

// IsSquare reports whether the block is square (needed to choose between
// certain partition types).
func (b BlockSize) IsSquare() bool {
	return blockWidths[b] == blockHeights[b]
}

var blockWidths = [BlockInvalid + 1]int{
	4, 4, 8, 8, 8, 16, 16, 16, 32, 32, 32, 64, 64, 64, 128, 128,
	4, 16, 8, 32, 16, 64, 0,
}

var blockHeights = [BlockInvalid + 1]int{
	4, 8, 4, 8, 16, 8, 16, 32, 16, 32, 64, 32, 64, 128, 64, 128,
	16, 4, 32, 8, 64, 16, 0,
}

// PartitionType enumerates the partition decisions at each node of the
// partition tree (spec §3 PARTITION_*).
type PartitionType uint8

const (
	PartitionNone   PartitionType = 0
	PartitionHorz   PartitionType = 1
	PartitionVert   PartitionType = 2
	PartitionSplit  PartitionType = 3
	PartitionHorzA  PartitionType = 4
	PartitionHorzB  PartitionType = 5
	PartitionVertA  PartitionType = 6
	PartitionVertB  PartitionType = 7
	PartitionHorz4  PartitionType = 8
	PartitionVert4  PartitionType = 9
)

// SubBlock is a leaf block emitted by the partition tree traversal.
type SubBlock struct {
	X, Y int
	Size BlockSize
}

// PartitionOracle is a callback that returns the partition decision for a
// square block at (x, y) of the given size. It must return PartitionNone
// once a further split would drop below 4x4 luma (smallest allowed block).
type PartitionOracle func(x, y int, bs BlockSize) PartitionType

// WalkPartition recursively walks the partition tree rooted at (x, y) with
// initial block size bs, calling oracle at each square node and emit on
// each leaf block. It implements the geometry of spec §5.11.4 but does
// NOT consume any bits — the oracle is expected to provide decisions from
// wherever the caller wants (tests, real bitstream, etc.).
func WalkPartition(x, y int, bs BlockSize, oracle PartitionOracle, emit func(SubBlock)) {
	if bs == Block4x4 {
		// Smallest possible block, no further splitting.
		emit(SubBlock{X: x, Y: y, Size: bs})
		return
	}
	pt := PartitionNone
	if bs.IsSquare() {
		pt = oracle(x, y, bs)
	}
	w := bs.Width()
	h := bs.Height()
	hw := w / 2
	hh := h / 2

	switch pt {
	case PartitionNone:
		emit(SubBlock{X: x, Y: y, Size: bs})
	case PartitionHorz:
		top := halfBelowSize(bs, true)
		bot := halfBelowSize(bs, true)
		emit(SubBlock{X: x, Y: y, Size: top})
		emit(SubBlock{X: x, Y: y + hh, Size: bot})
	case PartitionVert:
		left := halfBelowSize(bs, false)
		right := halfBelowSize(bs, false)
		emit(SubBlock{X: x, Y: y, Size: left})
		emit(SubBlock{X: x + hw, Y: y, Size: right})
	case PartitionSplit:
		sub := quarterSize(bs)
		WalkPartition(x, y, sub, oracle, emit)
		WalkPartition(x+hw, y, sub, oracle, emit)
		WalkPartition(x, y+hh, sub, oracle, emit)
		WalkPartition(x+hw, y+hh, sub, oracle, emit)
	default:
		// PartitionHorz4 / Vert4 / HorzA/B / VertA/B — not yet implemented.
		// Fall through to treat as a single leaf; a future milestone will
		// expand these correctly.
		emit(SubBlock{X: x, Y: y, Size: bs})
	}
}

// quarterSize returns the square block size obtained by splitting bs into
// four (spec §5.11.4 — PARTITION_SPLIT).
func quarterSize(bs BlockSize) BlockSize {
	switch bs {
	case Block128x128:
		return Block64x64
	case Block64x64:
		return Block32x32
	case Block32x32:
		return Block16x16
	case Block16x16:
		return Block8x8
	case Block8x8:
		return Block4x4
	}
	return BlockInvalid
}

// halfBelowSize returns the block size obtained by splitting bs along the
// given axis. axisIsHorz true means we split horizontally (stacked halves).
func halfBelowSize(bs BlockSize, axisIsHorz bool) BlockSize {
	if axisIsHorz {
		switch bs {
		case Block128x128:
			return Block128x64
		case Block64x64:
			return Block64x32
		case Block32x32:
			return Block32x16
		case Block16x16:
			return Block16x8
		case Block8x8:
			return Block8x4
		}
	} else {
		switch bs {
		case Block128x128:
			return Block64x128
		case Block64x64:
			return Block32x64
		case Block32x32:
			return Block16x32
		case Block16x16:
			return Block8x16
		case Block8x8:
			return Block4x8
		}
	}
	return BlockInvalid
}
