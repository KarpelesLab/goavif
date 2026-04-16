// Package filmgrain implements AV1 film grain synthesis (spec §7.20).
//
// Film grain is an optional post-processing pass that adds structured
// noise to decoded pixels, preserving the look of photographic grain
// after aggressive compression. The grain parameters (seed, scaling
// curve, AR coefficients) are signaled in the frame header's
// film_grain_params block; application is strictly a synthesis step
// that doesn't affect the reference frames.
//
// This package currently implements the seeded RNG and the piecewise-
// linear scaling curve lookup. AR-coefficient shaping and the per-
// block grain patch tiling are follow-up phases.
package filmgrain
