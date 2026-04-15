package cdfs

import "testing"

func TestMakeCDF2Validate(t *testing.T) {
	for _, p0 := range []uint16{100, 16000, 32767} {
		c := MakeCDF2(p0)
		if err := c.Validate(); err != nil {
			t.Errorf("MakeCDF2(%d).Validate = %v", p0, err)
		}
	}
}

func TestMakeCDF3(t *testing.T) {
	// Three equal probabilities sum to 32766 with a final symbol at 2.
	c := MakeCDF3(10922, 10922)
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// cdf[0] should equal 32768 - p0 = 21846; cdf[1] = 32768 - p0 - p1 = 10924.
	if c[0] != 21846 || c[1] != 10924 {
		t.Errorf("CDF3 = %v", c)
	}
}

func TestDefaultSkipCDFShape(t *testing.T) {
	for i, c := range DefaultSkipCDF {
		if err := c.Validate(); err != nil {
			t.Errorf("DefaultSkipCDF[%d]: %v", i, err)
		}
	}
	// Sanity: context 0 (both neighbors non-skip) should have the highest
	// P(skip=0), i.e., the smallest cdf[0] in the inverted representation.
	if DefaultSkipCDF[0][0] >= DefaultSkipCDF[2][0] {
		t.Errorf("DefaultSkipCDF ordering looks wrong: %v", DefaultSkipCDF)
	}
}

func TestValidateRejectsFlat(t *testing.T) {
	bad := CDF{100, 100, 0, 0}
	if err := bad.Validate(); err == nil {
		t.Errorf("flat CDF should fail validation")
	}
}

func TestValidateRejectsNonZeroSentinel(t *testing.T) {
	bad := CDF{1000, 500, 5, 0}
	if err := bad.Validate(); err == nil {
		t.Errorf("non-zero sentinel should fail validation")
	}
}
