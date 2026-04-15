package predict

// PaethPred fills dst with Paeth prediction (spec §7.11.2.4).
//
// For each sample at (r, c):
//
//	base = above[c] + left[r] - aboveLeft
//	p_a  = |base - above[c]|       // = |left[r] - aboveLeft|
//	p_l  = |base - left[r]|        // = |above[c] - aboveLeft|
//	p_al = |base - aboveLeft|
//	if  p_l <= p_a && p_l <= p_al: pred = left[r]
//	elif p_a <= p_al:               pred = above[c]
//	else:                           pred = aboveLeft
func PaethPred(dst []uint8, w, h int, above, left []uint8, aboveLeft uint8) {
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			a := int(above[c])
			l := int(left[r])
			al := int(aboveLeft)
			base := a + l - al
			pA := abs(base - a)
			pL := abs(base - l)
			pAL := abs(base - al)
			var p int
			switch {
			case pL <= pA && pL <= pAL:
				p = l
			case pA <= pAL:
				p = a
			default:
				p = al
			}
			if p < 0 {
				p = 0
			} else if p > 255 {
				p = 255
			}
			dst[r*w+c] = uint8(p)
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
