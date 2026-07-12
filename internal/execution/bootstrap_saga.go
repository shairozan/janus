package execution

import (
	"fmt"
	"path"
	"regexp"
	"sort"
)

// Horizontal PsN bootstrap on Kubernetes — the host-orchestrated saga (#192/#195).
//
// PsN's bootstrap is decomposed into three host-driven stages, each a single
// Hermes pod run (validated end-to-end in #193 on PsN 5.7.1):
//
//  1. SETUP    (PsN image): resample-only via the dummy compiler. Produces
//               <ws>/m1/bs_pr1_<i>.{mod,dta}. PsN's bootstrap exits non-zero
//               after the dummy "fails" every fit, but the datasets are written
//               first — so the host judges success by the produced files, not the
//               exit code (hence the "; true" tail on the script).
//  2. FITS     (execution image): one pod per resample, run by a FIFO worker
//               pool. Each ships its bs_pr1_<i>.{mod,dta} in and returns
//               bs_pr1_<i>.{lst,ext}.
//  3. AGGREGATE (PsN image): rebuild the workspace from the SETUP tree + the fit
//               outputs, then rawresults + bootstrap -summarize -> CIs.
//
// The desktop is the data hub between stages: it holds the workspace and ships
// the relevant slice into each pod (no shared/RWX PVC). This file holds the pure,
// orchestrator-free helpers (commands, file names, slicing); the wiring lives in
// the saga orchestrator.

// BootstrapWorkspaceDirName is the subdirectory, relative to the model directory,
// where a completed bootstrap saga's workspace is persisted on the host: the
// resampled datasets, every successful fit's outputs, and the aggregate result
// CSVs. The GUI zips this tree for the saga's "download all files".
const BootstrapWorkspaceDirName = bootstrapWorkspaceDir

const (
	// bootstrapWorkspaceDir is PsN's -directory, relative to the Hermes working
	// dir (/workspace). Kept short and stable so the returned file paths are
	// predictable.
	bootstrapWorkspaceDir = "bs"

	// bootstrapM1Subdir is PsN's hardcoded per-sample subdirectory under the
	// workspace, holding the resampled control files and datasets.
	bootstrapM1Subdir = "m1"

	// bootstrapDummyNMVersion is the psn.conf nm_version whose nmfe exits without
	// fitting, so SETUP resamples without running NONMEM (#193).
	bootstrapDummyNMVersion = "dummy"

	// resamplePrefix is the prefix PsN gives the per-sample control/data files
	// (e.g. bs_pr1_1.mod, bs_pr1_1.dta), confirmed on PsN 5.7.1.
	resamplePrefix = "bs_pr1_"
)

// resampleModelName / resampleDataName name the per-sample files PsN writes under
// <ws>/m1 during SETUP; fitListName / fitExtName name the outputs a fit produces.
func resampleModelName(i int) string { return fmt.Sprintf("%s%d.mod", resamplePrefix, i) }
func resampleDataName(i int) string  { return fmt.Sprintf("%s%d.dta", resamplePrefix, i) }
func fitListName(i int) string       { return fmt.Sprintf("%s%d.lst", resamplePrefix, i) }
func fitExtName(i int) string        { return fmt.Sprintf("%s%d.ext", resamplePrefix, i) }

// rawResultsFileName is the default raw-results filename PsN's -summarize reads,
// derived from the model's base name (e.g. "acop" -> "raw_results_acop.csv").
func rawResultsFileName(modelBase string) string {
	return fmt.Sprintf("raw_results_%s.csv", modelBase)
}

// m1Path joins a file name onto the workspace's m1 subdirectory using
// forward-slash (container) separators.
func m1Path(name string) string {
	return path.Join(bootstrapWorkspaceDir, bootstrapM1Subdir, name)
}

// bootstrapResampleScript builds the SETUP command (run as `bash -lc <script>`).
// The trailing "; true" normalizes the exit code: PsN exits non-zero after the
// dummy compiler "fails" each fit, but the resampled datasets are already on disk
// — success is judged by the produced files (see resampledIndices).
func bootstrapResampleScript(modelFile string, samples int) string {
	return fmt.Sprintf(
		"bootstrap %s -samples=%d -directory=%s -nm_version=%s -clean=0 -no-run_base_model; true",
		shellSingleQuote(modelFile), samples, bootstrapWorkspaceDir, bootstrapDummyNMVersion,
	)
}

// bootstrapAggregateScript builds the AGGREGATE command (run as `bash -lc`):
// remove any stale raw_results from SETUP, rebuild the parameter database with
// rawresults, then compute the CIs with bootstrap -summarize. rawresults refuses
// to overwrite, so the rm must precede it (#193).
func bootstrapAggregateScript(modelFile, modelBase string) string {
	out := path.Join(bootstrapWorkspaceDir, rawResultsFileName(modelBase))
	m1 := path.Join(bootstrapWorkspaceDir, bootstrapM1Subdir)

	return fmt.Sprintf(
		"rm -f %s/raw_results_*.csv && rawresults --path=%s --outfile=%s && bootstrap %s -directory=%s -summarize",
		bootstrapWorkspaceDir, m1, out, shellSingleQuote(modelFile), bootstrapWorkspaceDir,
	)
}

// fitArgs returns the command and args for a single fit pod (execution image).
// Hermes maps the logical "nonmem" command to the model's container nmfe; the
// control file and list file sit at the fit pod's workspace root.
func fitArgs(i int) (command string, args []string) {
	return "nonmem", []string{resampleModelName(i), fitListName(i)}
}

// fitRetain is the set of files a fit pod must return for aggregation: the list
// file and the parameter-estimate file.
func fitRetain(i int) []string {
	return []string{fitListName(i), fitExtName(i)}
}

// setupRetain is the set of files SETUP must return so the host can reconstruct
// the bootstrap workspace for AGGREGATE: everything under the workspace dir.
func setupRetain() []string {
	return []string{bootstrapWorkspaceDir + "/**"}
}

// aggregateRetain is the set of result files AGGREGATE returns to the host (and
// thence the model directory).
func aggregateRetain() []string {
	return []string{
		path.Join(bootstrapWorkspaceDir, "raw_results*.csv"),
		path.Join(bootstrapWorkspaceDir, "bootstrap_results.csv"),
	}
}

// resampleModelKeyRe matches a SETUP-returned control-file path of the form
// "<ws>/m1/bs_pr1_<i>.mod", capturing the sample index.
var resampleModelKeyRe = regexp.MustCompile(
	`(?:^|/)` + regexp.QuoteMeta(bootstrapM1Subdir) + `/` + regexp.QuoteMeta(resamplePrefix) + `(\d+)\.mod$`,
)

// resampledIndices returns the sorted sample indices for which SETUP produced a
// control file in the returned workspace. This is how the host judges that
// resampling succeeded (files, not exit code), and how many fits to fan out.
func resampledIndices(retained map[string][]byte) []int {
	var indices []int

	for key := range retained {
		m := resampleModelKeyRe.FindStringSubmatch(key)
		if m == nil {
			continue
		}

		n := 0
		for _, r := range m[1] {
			n = n*10 + int(r-'0')
		}

		indices = append(indices, n)
	}

	sort.Ints(indices)

	return indices
}

// fitInputs extracts the workspace files a single fit pod needs — its control
// file and resampled dataset — re-keyed to the fit pod's workspace root (bare
// names, where the control file's $DATA expects its sibling). It looks the files
// up under <ws>/m1 in the SETUP-returned set. Returns ok=false if either file is
// missing.
func fitInputs(setupFiles map[string][]byte, i int) (files map[string][]byte, ok bool) {
	modKey := lookupKeySuffix(setupFiles, m1Path(resampleModelName(i)))
	dataKey := lookupKeySuffix(setupFiles, m1Path(resampleDataName(i)))

	if modKey == "" || dataKey == "" {
		return nil, false
	}

	return map[string][]byte{
		resampleModelName(i): setupFiles[modKey],
		resampleDataName(i):  setupFiles[dataKey],
	}, true
}

// lookupKeySuffix returns the key in m equal to want or ending in "/"+want
// (tolerating an absent or present leading path component on returned file
// paths). Returns "" when no key matches.
func lookupKeySuffix(m map[string][]byte, want string) string {
	if _, ok := m[want]; ok {
		return want
	}

	suffix := "/" + want

	for k := range m {
		if len(k) > len(suffix) && k[len(k)-len(suffix):] == suffix {
			return k
		}
	}

	return ""
}

// shellSingleQuote wraps s in single quotes for safe inclusion in a `bash -lc`
// script, escaping any embedded single quotes. Model file names are simple, but
// this keeps the scripts robust.
func shellSingleQuote(s string) string {
	return "'" + reSingleQuote.ReplaceAllString(s, `'\''`) + "'"
}

var reSingleQuote = regexp.MustCompile(`'`)
