package obu

import "github.com/KarpelesLab/goavif/av1/bitio"

// SegmentationParams decodes segmentation_params() (spec §5.9.14).
type SegmentationParams struct {
	Enabled          bool
	UpdateMap        bool
	TemporalUpdate   bool
	UpdateData       bool
	FeatureEnabled   [MaxSegments][SegLvlMax]bool
	FeatureData      [MaxSegments][SegLvlMax]int16
	SegIDPreSkip     bool
	LastActiveSegID  uint8
}

func parseSegmentationParams(r *bitio.Reader, sp *SegmentationParams, fh *FrameHeader) {
	sp.Enabled = r.F(1) == 1
	if sp.Enabled {
		if fh.PrimaryRefFrame == PrimaryRefNone {
			sp.UpdateMap = true
			sp.TemporalUpdate = false
			sp.UpdateData = true
		} else {
			sp.UpdateMap = r.F(1) == 1
			if sp.UpdateMap {
				sp.TemporalUpdate = r.F(1) == 1
			}
			sp.UpdateData = r.F(1) == 1
		}
		if sp.UpdateData {
			for i := 0; i < MaxSegments; i++ {
				for j := 0; j < SegLvlMax; j++ {
					featureEnabled := r.F(1) == 1
					var clippedValue int16
					if featureEnabled {
						bits := segFeatureBits[j]
						if segFeatureSigned[j] {
							clippedValue = int16(r.Su(bits + 1))
						} else {
							clippedValue = int16(r.F(bits))
						}
					}
					sp.FeatureEnabled[i][j] = featureEnabled
					sp.FeatureData[i][j] = clippedValue
				}
			}
		}
	}

	// Derived fields:
	sp.SegIDPreSkip = false
	sp.LastActiveSegID = 0
	if sp.Enabled {
		for i := 0; i < MaxSegments; i++ {
			for j := 0; j < SegLvlMax; j++ {
				if sp.FeatureEnabled[i][j] {
					if uint8(i) > sp.LastActiveSegID {
						sp.LastActiveSegID = uint8(i)
					}
					if j >= SegLvlRefFrame {
						sp.SegIDPreSkip = true
					}
				}
			}
		}
	}
}
