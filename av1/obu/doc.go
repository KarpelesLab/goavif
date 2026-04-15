// Package obu implements parsing of AV1 Open Bitstream Units (spec §5.3).
// Consumers feed it OBU bytes extracted from an AVIF's av1C ConfigOBUs or
// from the mdat item data.
package obu
