package execution

import (
	"reflect"
	"strings"
	"testing"
)

func TestResampleFileNames(t *testing.T) {
	if got := resampleModelName(3); got != "bs_pr1_3.mod" {
		t.Errorf("resampleModelName(3) = %q", got)
	}

	if got := resampleDataName(3); got != "bs_pr1_3.dta" {
		t.Errorf("resampleDataName(3) = %q", got)
	}

	if got := fitListName(3); got != "bs_pr1_3.lst" {
		t.Errorf("fitListName(3) = %q", got)
	}

	if got := fitExtName(3); got != "bs_pr1_3.ext" {
		t.Errorf("fitExtName(3) = %q", got)
	}

	if got := rawResultsFileName("acop"); got != "raw_results_acop.csv" {
		t.Errorf("rawResultsFileName = %q", got)
	}
}

func TestBootstrapResampleScript(t *testing.T) {
	got := bootstrapResampleScript("acop.mod", 200)

	for _, want := range []string{
		"bootstrap 'acop.mod'", "-samples=200", "-directory=bs",
		"-nm_version=dummy", "-clean=0", "-no-run_base_model", "; true",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("resample script missing %q in %q", want, got)
		}
	}
}

func TestBootstrapAggregateScript(t *testing.T) {
	got := bootstrapAggregateScript("acop.mod", "acop")

	// rm must precede rawresults (it refuses to overwrite), which precedes summarize.
	rmIdx := strings.Index(got, "rm -f bs/raw_results_*.csv")
	rawIdx := strings.Index(got, "rawresults --path=bs/m1 --outfile=bs/raw_results_acop.csv")
	sumIdx := strings.Index(got, "bootstrap 'acop.mod' -directory=bs -summarize")

	if rmIdx < 0 || rawIdx < 0 || sumIdx < 0 {
		t.Fatalf("aggregate script missing a stage: %q", got)
	}

	if !(rmIdx < rawIdx && rawIdx < sumIdx) {
		t.Errorf("aggregate stages out of order: rm=%d raw=%d sum=%d", rmIdx, rawIdx, sumIdx)
	}
}

func TestResampledIndices(t *testing.T) {
	retained := map[string][]byte{
		"bs/m1/bs_pr1_1.mod":  {},
		"bs/m1/bs_pr1_1.dta":  {}, // data, not a model — ignored
		"bs/m1/bs_pr1_2.mod":  {},
		"bs/m1/bs_pr1_10.mod": {},
		"bs/meta.yaml":        {}, // not a control file
		"bs/m1/done.1":        {},
		"other/bs_pr1_9.mod":  {}, // not under m1 — ignored
	}

	got := resampledIndices(retained)
	want := []int{1, 2, 10}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("resampledIndices = %v, want %v", got, want)
	}
}

func TestResampledIndicesEmpty(t *testing.T) {
	if got := resampledIndices(map[string][]byte{"bs/meta.yaml": {}}); len(got) != 0 {
		t.Errorf("expected no indices, got %v", got)
	}
}

func TestFitInputs(t *testing.T) {
	setup := map[string][]byte{
		"bs/m1/bs_pr1_2.mod": []byte("$PROB"),
		"bs/m1/bs_pr1_2.dta": []byte("ID,DV"),
		"bs/m1/bs_pr1_1.mod": []byte("other"),
	}

	files, ok := fitInputs(setup, 2)
	if !ok {
		t.Fatal("expected ok for sample 2")
	}

	// Re-keyed to bare names at the fit pod workspace root.
	if string(files["bs_pr1_2.mod"]) != "$PROB" || string(files["bs_pr1_2.dta"]) != "ID,DV" {
		t.Errorf("fitInputs returned wrong content/keys: %v", keys(files))
	}

	if len(files) != 2 {
		t.Errorf("expected exactly 2 files, got %v", keys(files))
	}

	if _, ok := fitInputs(setup, 5); ok {
		t.Error("expected ok=false for missing sample 5")
	}

	// Missing dataset -> not ok.
	if _, ok := fitInputs(map[string][]byte{"bs/m1/bs_pr1_3.mod": {}}, 3); ok {
		t.Error("expected ok=false when dataset missing")
	}
}

func TestFitArgsAndRetain(t *testing.T) {
	cmd, args := fitArgs(7)
	if cmd != "nonmem" || !reflect.DeepEqual(args, []string{"bs_pr1_7.mod", "bs_pr1_7.lst"}) {
		t.Errorf("fitArgs(7) = %q %v", cmd, args)
	}

	if got := fitRetain(7); !reflect.DeepEqual(got, []string{"bs_pr1_7.lst", "bs_pr1_7.ext"}) {
		t.Errorf("fitRetain(7) = %v", got)
	}
}

func TestStageRetainPatterns(t *testing.T) {
	if got := setupRetain(); !reflect.DeepEqual(got, []string{"bs/**"}) {
		t.Errorf("setupRetain = %v", got)
	}

	if got := aggregateRetain(); !reflect.DeepEqual(got, []string{"bs/raw_results*.csv", "bs/bootstrap_results.csv"}) {
		t.Errorf("aggregateRetain = %v", got)
	}
}

func TestShellSingleQuote(t *testing.T) {
	if got := shellSingleQuote("acop.mod"); got != "'acop.mod'" {
		t.Errorf("got %q", got)
	}

	if got := shellSingleQuote("a'b"); got != `'a'\''b'` {
		t.Errorf("embedded quote: got %q", got)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}
