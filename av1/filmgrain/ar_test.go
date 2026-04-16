package filmgrain

import (
	"testing"
)

func TestGenerateGrainTemplateSameSeedDeterministic(t *testing.T) {
	a := GenerateGrainTemplate(16, 16, 0x1234)
	b := GenerateGrainTemplate(16, 16, 0x1234)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("template[%d] differs: %d vs %d", i, a[i], b[i])
		}
	}
}

func TestGenerateGrainTemplateRangeIsSignedByte(t *testing.T) {
	g := GenerateGrainTemplate(32, 32, 0xABCD)
	for i, v := range g {
		if v < -128 || v > 127 {
			t.Fatalf("sample %d = %d escaped [-128,127]", i, v)
		}
	}
}

func TestApplyARZeroLagIsNoop(t *testing.T) {
	g := GenerateGrainTemplate(8, 8, 0xBEEF)
	save := append([]int16(nil), g...)
	ApplyAR(g, 8, 8, 0, nil, 6)
	for i := range g {
		if g[i] != save[i] {
			t.Fatalf("zero-lag changed sample %d", i)
		}
	}
}

func TestApplyARCoeffCountMismatchIsNoop(t *testing.T) {
	g := GenerateGrainTemplate(8, 8, 0xCAFE)
	save := append([]int16(nil), g...)
	// Pass only 3 coeffs while lag=2 expects (2*2+1)*2 + 2 = 12.
	ApplyAR(g, 8, 8, 2, []int8{1, 2, 3}, 7)
	for i := range g {
		if g[i] != save[i] {
			t.Fatalf("mismatched-coeff-count changed sample %d", i)
		}
	}
}

func TestApplyARZeroCoeffsDoesNotShape(t *testing.T) {
	g := GenerateGrainTemplate(16, 16, 0x5555)
	save := append([]int16(nil), g...)
	// Lag 3 taps = (2*3+1)*3 + 3 = 24.
	ApplyAR(g, 16, 16, 3, make([]int8, 24), 7)
	// Zero coeffs should leave every sample untouched (sum=0 → add 0
	// after the rounded divide).
	for i := range g {
		if g[i] != save[i] {
			t.Fatalf("zero-coeff AR modified sample %d: %d -> %d", i, save[i], g[i])
		}
	}
}

func TestApplyARNonZeroCoeffsChangeSomething(t *testing.T) {
	g := GenerateGrainTemplate(16, 16, 0x9999)
	save := append([]int16(nil), g...)
	// Lag 1 taps = (2*1+1)*1 + 1 = 4.
	ApplyAR(g, 16, 16, 1, []int8{10, 20, 30, 40}, 6)
	diffs := 0
	for i := range g {
		if g[i] != save[i] {
			diffs++
		}
	}
	if diffs == 0 {
		t.Fatal("non-zero coeffs produced no changes")
	}
}

func TestApplyARResultsInBounds(t *testing.T) {
	g := GenerateGrainTemplate(32, 32, 0xA5A5)
	// Large coeffs to stress the clip path.
	coeffs := make([]int8, (2*3+1)*3+3)
	for i := range coeffs {
		coeffs[i] = 127
	}
	ApplyAR(g, 32, 32, 3, coeffs, 6)
	for i, v := range g {
		if v < -2048 || v > 2047 {
			t.Fatalf("clipped range violated at %d: %d", i, v)
		}
	}
}
