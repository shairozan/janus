package pirana

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/shairozan/janus/internal/config"
)

// settingsFromKV extracts a Settings from a flattened, lower-cased key→value
// map harvested from a Pirana settings file. Keys are matched defensively by
// substring because their exact names differ across Pirana versions and across
// the SQLite and INI storage formats.
func settingsFromKV(kv map[string]string) *Settings {
	s := &Settings{}

	s.NonmemPath = firstMatch(kv,
		"nm_installation_path", "nonmem_path", "nonmempath", "nm_path", "installation_path")

	s.NonmemBinary = normalizeBinary(firstMatch(kv,
		"nonmem_binary", "nm_version", "nm_binary", "nmfe"))

	s.DefaultDir = firstMatch(kv,
		"default_dir", "default-directory", "default_directory", "start_in", "start_folder",
		"models_dir", "model_dir", "pirana_dir", "project_dir", "working_dir")

	s.Scheduler = normalizeScheduler(firstMatch(kv,
		"grid_type", "scheduler", "workload_manager", "cluster_type"))

	s.Researcher = firstMatch(kv,
		"name_of_researcher", "researcher", "author")

	s.AutoBackup = firstBool(kv, "auto_backup", "backup_models", "backup")
	s.AutoCleanup = firstBool(kv, "auto_cleanup", "cleanup_folders", "cleanup")
	s.DisallowOverwrite = firstBool(kv, "disallow_overwrite", "overwrite")

	s.RunPrefix = firstMatch(kv, "prefix_for_models", "run_prefix", "model_prefix", "prefix")
	s.AltDataDir = firstMatch(kv, "alternative_data", "alt_data", "data_directory")
	s.CloseConsole = firstBool(kv, "close_console", "close_window")

	s.RemoteHost = firstMatch(kv, "remote_host", "ssh_host")
	s.RemoteUser = firstMatch(kv, "remote_user", "ssh_user")
	s.RemoteMountLocal = firstMatch(kv, "local_mount")
	s.RemoteMountRemote = firstMatch(kv, "remote_mount")

	s.PSNPath = firstMatch(kv, "psn_path", "psn_installation")
	s.PSNConf = firstMatch(kv, "psn_conf", "psn_config")

	s.RPath = firstMatch(kv, "rscript_path", "rscript", "r_location", "r_home")
	s.Tools = detectTools(kv)
	s.Installations = installationsFromKV(kv)

	s.Detected = detectUnmapped(kv)

	return s
}

// installationsFromKV reconstructs the list of NONMEM installations emitted by
// the readers as indexed `nm_installation.<i>.{path,name,version}` keys. The
// first is marked the default.
func installationsFromKV(kv map[string]string) []config.NonmemInstall {
	var installs []config.NonmemInstall

	for i := 0; ; i++ {
		path := strings.TrimSpace(kv[fmt.Sprintf("nm_installation.%d.path", i)])
		if path == "" {
			break
		}

		name := strings.TrimSpace(kv[fmt.Sprintf("nm_installation.%d.name", i)])
		if name == "" {
			name = fmt.Sprintf("nm%d", i+1)
		}

		installs = append(installs, config.NonmemInstall{
			Name:    name,
			Path:    path,
			Binary:  normalizeBinary(kv[fmt.Sprintf("nm_installation.%d.version", i)]),
			Default: i == 0,
		})
	}

	return installs
}

// detectTools extracts named external-tool paths (Stan, NMQual, WFN) registered
// in the Pirana settings, in a deterministic order.
func detectTools(kv map[string]string) []config.IntegrationTool {
	specs := []struct {
		name    string
		needles []string
	}{
		{"Stan", []string{"stan_dir", "stan_path", "stan"}},
		{"NMQual", []string{"nmqual"}},
		{"WFN", []string{"wfn"}},
	}

	var tools []config.IntegrationTool

	for _, spec := range specs {
		if p := firstMatch(kv, spec.needles...); p != "" {
			tools = append(tools, config.IntegrationTool{Name: spec.name, Path: p})
		}
	}

	return tools
}

// hasToolNamed reports whether tools already contains a tool with the given name.
func hasToolNamed(tools []config.IntegrationTool, name string) bool {
	for _, t := range tools {
		if strings.EqualFold(t.Name, name) {
			return true
		}
	}

	return false
}

// firstBool returns a parsed boolean for the first key (in lexical order) that
// contains one of the needles and whose value parses as a boolean; nil when no
// such key exists, so callers can distinguish "absent" from "false".
func firstBool(kv map[string]string, needles ...string) *bool {
	keys := sortedKeys(kv)

	for _, needle := range needles {
		for _, k := range keys {
			if !strings.Contains(k, needle) {
				continue
			}

			if b, ok := parseBool(kv[k]); ok {
				return &b
			}
		}
	}

	return nil
}

// parseBool interprets common boolean spellings found in Pirana settings.
func parseBool(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on", "y", "t":
		return true, true
	case "0", "false", "no", "off", "n", "f":
		return false, true
	default:
		return false, false
	}
}

// defaultCleanupGlobs is a conservative set of NONMEM scratch files removed when
// Pirana's "automatically cleanup folders" preference was enabled.
func defaultCleanupGlobs() []string {
	return []string{
		"FDATA", "FCON", "FREPORT", "FSTREAM", "FMSG", "FSUBS*",
		"PRDERR", "INTER", "OUTPUT", "nonmem.exe",
	}
}

// firstMatch returns the value of the first key (in lexical order, for
// determinism) that contains one of the given needles as a case-insensitive
// substring and has a non-empty value. Needles are tried in priority order.
func firstMatch(kv map[string]string, needles ...string) string {
	keys := sortedKeys(kv)

	for _, needle := range needles {
		for _, k := range keys {
			if kv[k] == "" {
				continue
			}

			if strings.Contains(k, needle) {
				return kv[k]
			}
		}
	}

	return ""
}

// normalizeScheduler maps a Pirana grid/workload-manager identifier onto a
// Janus scheduler value. Unknown or local values return "" so the caller can
// leave the Janus default in place.
func normalizeScheduler(grid string) string {
	switch strings.ToLower(strings.TrimSpace(grid)) {
	case "slurm":
		return "SLURM"
	case "sge", "gridengine", "grid engine":
		return "SGE"
	case "torque", "jsub-torque", "jsub_torque", "pbs", "pbs-torque":
		return "TORQUE"
	default:
		return ""
	}
}

var versionDigitsRe = regexp.MustCompile(`\d+`)

// normalizeBinary derives a Janus nonmem-binary (e.g. "nmfe75") from a Pirana
// value, which may already be a binary name ("nmfe74", "nmfe74.bat"), a bare
// version ("7.5", "75"), or empty. It returns "" when no version can be found.
func normalizeBinary(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" {
		return ""
	}

	// Already an nmfe binary: strip any extension (e.g. ".bat") and return.
	if strings.HasPrefix(v, "nmfe") {
		return strings.TrimSuffix(v, filepath.Ext(v))
	}

	// Bare version: take the first two numeric groups (7.4.0 -> 74, 7.5 -> 75).
	parts := versionDigitsRe.FindAllString(v, -1)
	if len(parts) == 0 {
		return ""
	}

	if len(parts) > 2 {
		parts = parts[:2]
	}

	return "nmfe" + strings.Join(parts, "")
}

// unmappedKeys describes Pirana settings that are meaningful but have no Janus
// destination in this version. When one is present it is reported (by label) so
// the user knows it was seen but not imported.
var unmappedKeys = []struct {
	needle string
	label  string
}{
	{"darwin", "pyDarwin settings"},
	{"nlme", "NLME / RsNLME settings"},
}

// detectUnmapped returns a de-duplicated, ordered list of labels for known
// Pirana settings that were found but are not imported.
func detectUnmapped(kv map[string]string) []string {
	seen := map[string]bool{}

	var out []string

	for _, k := range sortedKeys(kv) {
		if kv[k] == "" {
			continue
		}

		for _, u := range unmappedKeys {
			if strings.Contains(k, u.needle) && !seen[u.label] {
				seen[u.label] = true
				out = append(out, u.label)
			}
		}
	}

	return out
}

// sortedKeys returns the keys of kv in lexical order.
func sortedKeys(kv map[string]string) []string {
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
