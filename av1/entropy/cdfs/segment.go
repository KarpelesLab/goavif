package cdfs

// DefaultSpatialPredSegTreeCDF is default_spatial_pred_seg_tree_cdf from
// libaom entropymode.c. Indexed by one of SPATIAL_PREDICTION_PROBS (3)
// context buckets derived from neighbor segment ids, each encoding the
// current segment id in MAX_SEGMENTS (8) symbols.
var DefaultSpatialPredSegTreeCDF [3]CDF

func init() {
	DefaultSpatialPredSegTreeCDF[0] = AomCDF(5622, 7893, 16093, 18233, 27809, 28373, 32533)
	DefaultSpatialPredSegTreeCDF[1] = AomCDF(14274, 18230, 22557, 24935, 29980, 30851, 32344)
	DefaultSpatialPredSegTreeCDF[2] = AomCDF(27527, 28487, 28723, 28890, 32397, 32647, 32679)
}
