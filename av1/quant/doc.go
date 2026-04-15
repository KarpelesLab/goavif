// Package quant implements the quantization pipeline for AV1 (spec §7.12).
//
// The AV1 decoder uses two 256-entry lookup tables (dc_q_lookup and
// ac_q_lookup), one pair for each bit depth (8/10/12). Given the
// uncompressed header's base_q_index, any delta_q_y_dc / delta_q_u_dc /
// delta_q_u_ac / delta_q_v_dc / delta_q_v_ac values, and a segment's
// alt-q feature (if enabled), the decoder computes per-plane DC and AC
// dequantization values and applies them to the dequantized coefficients
// before the inverse transform.
package quant
