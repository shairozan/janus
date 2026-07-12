#!/bin/bash
# Janus K8s bootstrap spike (issue #193): validate PsN's staged decomposition
# using the faked NONMEM in this image — no real NONMEM / license required.
# VALIDATED on PsN 5.7.1.
#
# Stages mirror the planned saga (#192):
#   1.  resample-only via the dummy compiler            (host SETUP pod)
#   2.  simulate the external fits                      (cluster fit pods)
#   3a. rawresults over m1/                             (host AGGREGATE pod, part 1)
#   3b. bootstrap -summarize -> CIs                     (host AGGREGATE pod, part 2)

set -uo pipefail
cd /work

MODEL="${MODEL:-acop.mod}"
MODELBASE="$(basename "${MODEL%.mod}")"
SAMPLES="${SAMPLES:-20}"        # >=19 so the 90% (5%/95%) percentile CI computes
WS="${WS:-ws_spike}"
MOCK=/opt/mock-nonmem
rm -rf "$WS"

pass=0; fail=0
hr(){ printf '\n========== %s ==========\n' "$1"; }
ck(){ # ck "label" <test-exit-status>
    if [ "$2" -eq 0 ]; then echo "PASS: $1"; pass=$((pass+1));
    else echo "FAIL: $1"; fail=$((fail+1)); fi; }

hr "PsN version"; psn --version 2>&1 | head -1

# --- Stage 1: resample-only (dummy compiler short-circuits the fits) ----------
hr "Stage 1: resample-only (-nm_version=dummy) — no fits"
bootstrap "$MODEL" -samples="$SAMPLES" -directory="$WS" \
    -nm_version=dummy -clean=0 -no-run_base_model >/dev/null 2>&1 || true
mods=$(ls "$WS"/m1/bs_pr1_*.mod 2>/dev/null | wc -l)
dtas=$(ls "$WS"/m1/bs_pr1_*.dta 2>/dev/null | wc -l)
lst0=$(ls "$WS"/m1/bs_pr1_*.lst 2>/dev/null | wc -l)
echo "control files: $mods   resampled datasets: $dtas   stray .lst: $lst0   (expected $SAMPLES / $SAMPLES / 0)"
[ "$mods" -eq "$SAMPLES" ]; ck "resample produced $SAMPLES control files (bs_pr1_N.mod)" $?
[ "$dtas" -eq "$SAMPLES" ]; ck "resample produced $SAMPLES datasets (bs_pr1_N.dta)" $?
[ "$lst0" -eq 0 ];          ck "no fits were run during resample (no .lst)" $?

# --- Stage 2: simulate the external fits (host would fan these to the cluster) -
hr "Stage 2: simulate external fits (drop canned acop .lst/.ext per sample)"
for m in "$WS"/m1/bs_pr1_*.mod; do
    b="${m%.mod}"; cp "$MOCK/acop.lst" "$b.lst"; cp "$MOCK/acop.ext" "$b.ext"
done
lsts=$(ls "$WS"/m1/bs_pr1_*.lst 2>/dev/null | wc -l)
[ "$lsts" -eq "$SAMPLES" ]; ck "produced $SAMPLES completed .lst" $?

# --- Stage 3a: rawresults builds the parameter database from the completed runs
hr "Stage 3a: rawresults --path=m1 -> raw_results_${MODELBASE}.csv"
rm -f "$WS"/raw_results_*.csv     # rawresults refuses to overwrite an existing outfile
rawresults --path="$WS/m1" --outfile="$WS/raw_results_${MODELBASE}.csv" 2>&1 | tail -2
valid=$(grep -c 2636.84 "$WS/raw_results_${MODELBASE}.csv" 2>/dev/null || true)
echo "valid result rows: $valid"
[ "$valid" -eq "$SAMPLES" ]; ck "rawresults parsed $SAMPLES completed runs" $?

# --- Stage 3b: bootstrap -summarize computes the CIs from raw_results ----------
hr "Stage 3b: bootstrap -summarize -> bootstrap_results.csv"
bootstrap "$MODEL" -directory="$WS" -summarize 2>&1 | tail -5
[ -f "$WS/bootstrap_results.csv" ]; ck "bootstrap_results.csv produced" $?
grep -q 'percentile.confidence.intervals' "$WS/bootstrap_results.csv" 2>/dev/null
ck "percentile CIs present" $?

# --- Verdict ------------------------------------------------------------------
hr "Summary"
echo "PASS=$pass  FAIL=$fail"
echo "Min-runs (PsN): 90% CI(5%/95%) needs N>=19; 95%(2.5/97.5) N>=39; 99% N>=199."
if [ "$fail" -eq 0 ]; then
    echo "RESULT: GO — PsN staged decomposition validated with faked NONMEM."
else
    echo "RESULT: NO-GO — see FAIL lines above."
fi
exit 0
