// Command goavif-info prints a summary of an AVIF file's container
// structure: ftyp brands, items, properties, and the primary item's
// dimensions and color configuration.
//
// It does not decode pixels — that's the job of the main library once
// the AV1 pixel decode path lands. This tool is useful today for
// inspecting AVIFs produced by other encoders or for debugging container
// changes.
package main

import (
	"fmt"
	"os"

	"github.com/KarpelesLab/goavif/av1/obu"
	"github.com/KarpelesLab/goavif/isobmff"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: goavif-info FILE")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		die(err)
	}
	ct, err := isobmff.ParseContainer(data)
	if err != nil {
		die(err)
	}
	if err := dumpContainer(ct); err != nil {
		die(err)
	}
}

func dumpContainer(ct *isobmff.Container) error {
	fmt.Printf("ftyp: major=%s minor=%d\n", ct.Ftyp.MajorBrand, ct.Ftyp.MinorVersion)
	fmt.Printf("  compatible_brands:")
	for _, b := range ct.Ftyp.CompatibleBrands {
		fmt.Printf(" %s", b)
	}
	fmt.Println()

	fmt.Printf("mdat: %d bytes at offset %d\n",
		func() int {
			if ct.Mdat == nil {
				return 0
			}
			return len(ct.Mdat.Data)
		}(),
		ct.MdatOffset,
	)

	if ct.Meta == nil {
		fmt.Println("meta: <absent>")
		return nil
	}
	primary := ct.PrimaryItemID()
	fmt.Printf("primary_item: %d\n", primary)

	iinf := findIinf(ct.Meta)
	if iinf != nil {
		fmt.Println("items:")
		for _, e := range iinf.Entries {
			fmt.Printf("  id=%d type=%s name=%q\n", e.ItemID, e.ItemType, e.ItemName)
		}
	}

	iprp := findIprp(ct.Meta)
	if iprp != nil {
		fmt.Println("properties (ipco):")
		for i, p := range iprp.Ipco.Properties {
			fmt.Printf("  [%d] %s", i+1, p.BoxType())
			describeProp(p)
		}
		fmt.Println("associations (ipma):")
		for _, m := range iprp.Ipma {
			for _, e := range m.Entries {
				fmt.Printf("  item=%d:", e.ItemID)
				for _, a := range e.Associations {
					essential := ""
					if a.Essential {
						essential = "*"
					}
					fmt.Printf(" %d%s", a.PropertyIndex, essential)
				}
				fmt.Println()
			}
		}
	}

	if primary != 0 {
		if err := dumpPrimaryAV1Info(ct, primary); err != nil {
			fmt.Printf("primary av1 info: %v\n", err)
		}
	}
	return nil
}

func describeProp(p isobmff.Box) {
	switch v := p.(type) {
	case *isobmff.Ispe:
		fmt.Printf(" %dx%d\n", v.Width, v.Height)
	case *isobmff.Pixi:
		fmt.Printf(" channels=%v\n", v.ChannelBits)
	case *isobmff.Av1C:
		fmt.Printf(" profile=%d level=%d high_bd=%d 12bit=%d mono=%d sub=%d/%d obus=%db\n",
			v.SeqProfile, v.SeqLevelIdx0, v.HighBitdepth, v.TwelveBit,
			v.Monochrome, v.ChromaSubsamplingX, v.ChromaSubsamplingY,
			len(v.ConfigOBUs))
	case *isobmff.Colr:
		if v.Type == isobmff.ColrTypeNCLX {
			fmt.Printf(" nclx cp=%d tc=%d mc=%d full=%v\n",
				v.ColourPrimaries, v.TransferCharacteristics,
				v.MatrixCoefficients, v.FullRange)
		} else {
			fmt.Printf(" %s icc=%db\n", v.Type, len(v.ICC))
		}
	case *isobmff.AuxC:
		fmt.Printf(" type=%q\n", v.AuxType)
	case *isobmff.Irot:
		fmt.Printf(" angle=%d*90\n", v.Angle)
	case *isobmff.Imir:
		fmt.Printf(" axis=%d\n", v.Axis)
	default:
		fmt.Println()
	}
}

func dumpPrimaryAV1Info(ct *isobmff.Container, itemID uint32) error {
	iprp := findIprp(ct.Meta)
	if iprp == nil {
		return fmt.Errorf("no iprp")
	}
	for _, m := range iprp.Ipma {
		for _, e := range m.Entries {
			if e.ItemID != itemID {
				continue
			}
			for _, a := range e.Associations {
				if a.PropertyIndex == 0 || int(a.PropertyIndex) > len(iprp.Ipco.Properties) {
					continue
				}
				if av1c, ok := iprp.Ipco.Properties[a.PropertyIndex-1].(*isobmff.Av1C); ok {
					sh, err := parseAV1CSeqHeader(av1c)
					if err != nil {
						return err
					}
					fmt.Printf("av1 sequence header:\n")
					fmt.Printf("  still_picture=%v reduced=%v\n", sh.StillPicture, sh.ReducedStillPictureHeader)
					fmt.Printf("  max_dims=%dx%d\n", sh.MaxFrameWidthMinusOne+1, sh.MaxFrameHeightMinusOne+1)
					fmt.Printf("  bit_depth=%d num_planes=%d mono=%v\n",
						sh.Color.BitDepth, sh.Color.NumPlanes, sh.Color.Monochrome)
					fmt.Printf("  subsampling=(%d,%d) chroma_pos=%d range=%v\n",
						sh.Color.SubsamplingX, sh.Color.SubsamplingY,
						sh.Color.ChromaSamplePosition, sh.Color.ColorRange)
					fmt.Printf("  tools: cdef=%v restoration=%v superres=%v film_grain=%v\n",
						sh.EnableCdef, sh.EnableRestoration, sh.EnableSuperres,
						sh.FilmGrainParamsPresent)
				}
			}
		}
	}
	return nil
}

func parseAV1CSeqHeader(av1c *isobmff.Av1C) (*obu.SequenceHeader, error) {
	obus, err := obu.Split(av1c.ConfigOBUs)
	if err != nil {
		return nil, err
	}
	for _, u := range obus {
		if u.Header.Type == obu.TypeSequenceHeader {
			return obu.ParseSequenceHeader(u.Payload)
		}
	}
	return nil, fmt.Errorf("no sequence header OBU in av1C")
}

func findIinf(m *isobmff.Meta) *isobmff.Iinf {
	for _, c := range m.Children {
		if i, ok := c.(*isobmff.Iinf); ok {
			return i
		}
	}
	return nil
}

func findIprp(m *isobmff.Meta) *isobmff.Iprp {
	for _, c := range m.Children {
		if i, ok := c.(*isobmff.Iprp); ok {
			return i
		}
	}
	return nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
