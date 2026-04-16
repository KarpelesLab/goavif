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

func TestDefaultPartitionCDFValidate(t *testing.T) {
	for i, c := range DefaultPartitionCDF {
		if err := c.Validate(); err != nil {
			t.Errorf("DefaultPartitionCDF[%d]: %v", i, err)
		}
	}
}

func TestDefaultKfYModeCDFValidate(t *testing.T) {
	for a := 0; a < 5; a++ {
		for l := 0; l < 5; l++ {
			c := DefaultKfYModeCDF[a][l]
			if err := c.Validate(); err != nil {
				t.Errorf("DefaultKfYModeCDF[%d][%d]: %v", a, l, err)
			}
		}
	}
}

func TestDefaultUVModeCDFValidate(t *testing.T) {
	for cfl := 0; cfl < 2; cfl++ {
		for ym := 0; ym < 13; ym++ {
			c := DefaultUVModeCDF[cfl][ym]
			if err := c.Validate(); err != nil {
				t.Errorf("DefaultUVModeCDF[%d][%d]: %v", cfl, ym, err)
			}
		}
	}
}

func TestDefaultAngleDeltaCDFValidate(t *testing.T) {
	for i, c := range DefaultAngleDeltaCDF {
		if err := c.Validate(); err != nil {
			t.Errorf("DefaultAngleDeltaCDF[%d]: %v", i, err)
		}
	}
}

func TestDefaultTxSizeCDFValidate(t *testing.T) {
	for cat := 0; cat < 4; cat++ {
		for ctx := 0; ctx < 3; ctx++ {
			c := DefaultTxSizeCDF[cat][ctx]
			if err := c.Validate(); err != nil {
				t.Errorf("DefaultTxSizeCDF[%d][%d]: %v", cat, ctx, err)
			}
		}
	}
}

func TestDefaultTxfmPartitionCDFValidate(t *testing.T) {
	for i, c := range DefaultTxfmPartitionCDF {
		if err := c.Validate(); err != nil {
			t.Errorf("DefaultTxfmPartitionCDF[%d]: %v", i, err)
		}
	}
}

func TestAomCDFInversion(t *testing.T) {
	c := AomCDF(19132, 25510, 30392)
	// Stored values should be 32768 - raw.
	if c[0] != 32768-19132 || c[1] != 32768-25510 || c[2] != 32768-30392 {
		t.Errorf("AomCDF inversion: got %v", c)
	}
	if c[3] != 0 || c[4] != 0 {
		t.Errorf("sentinel/count: got %d, %d", c[3], c[4])
	}
}
