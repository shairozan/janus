package config

// ResolveRetain picks the effective retain globs by inheritance: the first
// non-empty of the per-model list, the global default, and a caller-supplied
// fallback (the category default, or DefaultNONMEMRetain). It returns a fresh
// slice and never mutates its inputs.
//
// The fallback is passed in rather than resolved here because config is the
// lowest layer and cannot import the category package (which imports config).
func ResolveRetain(perModel, global, fallback []string) []string {
	switch {
	case len(perModel) > 0:
		return append([]string(nil), perModel...)
	case len(global) > 0:
		return append([]string(nil), global...)
	default:
		return append([]string(nil), fallback...)
	}
}

// LoadModelRetain returns the model's own retain list from its .janus.config.json,
// or nil when there is no config, it can't be read, or it declares no retain. It
// uses the low-level LoadModelConfig (no Hermes validation), so a non-Hermes model
// can declare its output files without an image/resources block.
func LoadModelRetain(modelPath string) []string {
	cfg, err := LoadModelConfig(modelPath)
	if err != nil || cfg == nil {
		return nil
	}

	return cfg.Retain
}
