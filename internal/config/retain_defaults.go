package config

// DefaultNONMEMRetain is the best-practice set of output-file glob patterns kept
// after a NONMEM run. It is the single source of truth for NONMEM retention: it
// seeds new .janus.config.json files (CLI `hermes init` and the GUI dialogs) and
// is the runtime last-resort fallback via
// category.NONMEMCategory.RetentionTargets().
//
// A fresh slice is returned on every call so callers that append/mutate cannot
// corrupt the canonical value.
func DefaultNONMEMRetain() []string {
	return []string{
		"*.lst", "*.ext", "*.xml", "*.phi",
		"*.cov", "*.cor", "*.coi", "*.shk",
		"*.shm", "*.grd", "*.cpu",
		"sdtab*", "patab*", "cotab*", "catab*",
	}
}

// DefaultPSNRetain is the retention for a PsN-tool run: the whole PsN run
// directory (PsN nests its NONMEM runs and result files under it). It mirrors the
// execution layer's psnHermesRetain so callers that need the default without
// importing the execution package can reach it.
func DefaultPSNRetain() []string {
	return []string{"psn_janus/**"}
}
