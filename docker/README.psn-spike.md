# PsN K8s bootstrap spike (#193)

A throwaway, fully-local rig to answer the **gating question** for the
host-orchestrated K8s bootstrap (epic #192): can PsN's `bootstrap` be split into

1. **resample-only** (build the N datasets + control files without running fits),
2. *(host runs the fits externally — simulated here)*,
3. **aggregate** from completed runs (`rawresults` + `bootstrap -summarize`),

…with **no real NONMEM and no license**? The image bundles PsN with a *faked*
NONMEM (a dummy compiler for the resample step + the existing mock `nmfe` for
simulated fits).

## Build

```bash
docker build -f docker/Dockerfile.psn-spike -t janus-psn-spike:latest .
# Pin a real PsN release (>= 4.6.0) if the default fails:
#   --build-arg PSN_VERSION=5.3.1
```

## Run

```bash
docker run --rm janus-psn-spike:latest            # runs Tests 1-3, prints results
docker run --rm -it janus-psn-spike:latest bash   # interactive; then: psn-spike
```

## Result: GO (validated on PsN 5.7.1)

The staged decomposition works. The exact, confirmed command sequence (feeds the
saga implementation in #194/#195):

```bash
# 1. SETUP pod — resample only, no fits. Produces m1/bs_pr1_<i>.{mod,dta}.
bootstrap acop.mod -samples=N -directory=ws -nm_version=dummy -clean=0 -no-run_base_model

# 2. FIT pods (host fans out) — each produces bs_pr1_<i>.{lst,ext} back into m1/.

# 3a. AGGREGATE pod — build the parameter database. rawresults refuses to
#     overwrite, so remove any stale file first. Output MUST be named
#     raw_results_<modelbase>.csv so -summarize finds it.
rm -f ws/raw_results_acop.csv
rawresults --path=ws/m1 --outfile=ws/raw_results_acop.csv

# 3b. AGGREGATE pod — compute the CIs.
bootstrap acop.mod -directory=ws -summarize        # -> ws/bootstrap_results.csv
```

### Confirmed findings (for #194 / #195)
- **File naming:** resampled control+data are `m1/bs_pr1_<i>.mod` + `bs_pr1_<i>.dta`
  (flat in `m1/`), *not* the `m1_i.mod`/`psn_bootstrap_data_i.csv` the research guessed.
- **Dummy short-circuit:** PsN logs "NMtran could not be initiated … no output for
  model N" per sample and exits non-zero after writing the datasets — i.e. the
  SETUP `bootstrap` call is expected to error at the end; the datasets are intact.
  The saga should treat SETUP as "succeeded if `bs_pr1_*.{mod,dta}` exist", not on
  exit code.
- **rawresults options:** `--path=<dir>` and `--outfile=<file>` (not `-directory`).
  It will not overwrite an existing outfile.
- **-summarize input:** reads `raw_results_<modelbase>.csv` (the default name). A
  stale all-NA file from the SETUP run will crash it (`bootstrap.pm:1565`), so the
  AGGREGATE step must (re)write that exact file from the real fits first.
- **Min-runs guardrail confirmed empirically:** at N=20 the 90% CI (5%/95%)
  computes; 2.5%/97.5% are `NA` until N≥39, 0.5%/99.5% until N≥199 — matches PsN's
  documented thresholds. Surface this in the UI/validation.
- **No NONMEM license needed** for SETUP (dummy) or AGGREGATE (`-summarize`).
- **Guardrails:** PsN ≥ 4.6.0; percentile CIs only (no `-bca`).

A NO-GO on Stage 1 would have meant pursuing the `ud` scheduler + relay fallback
(documented on #192) — not needed.

## Production image (for the real saga)

This spike image (perl base, fake NONMEM producer, sample model) is for *local
validation only*. The saga's SETUP/AGGREGATE pods are **Hermes** pods, so the
real PsN orchestration image is **`docker/Dockerfile.psn`** — the same PsN install
rebased onto the Hermes server base, with only the dummy compiler (no fake
producer, no sample model, no NONMEM/license). It's freely publishable. Build,
push, and point `hermes.psn_image` at it; the fits run on your own NONMEM
execution image. See that Dockerfile's header for details.

## Files
- `docker/Dockerfile.psn` — **production** PsN orchestration image (Hermes + PsN + dummy compiler).
- `docker/Dockerfile.psn-spike` — local spike image (PsN + faked NONMEM).
- `docker/psn.conf` — registers the `default` (mock) and `dummy` nm_versions.
- `docker/psn-spike.sh` — runs Tests 1-3.
- `testdata/mock-nonmem/dummy-nonmem.sh` — the resample-only dummy compiler.
- Reuses `testdata/mock-nonmem/` (mock `nmfe`, canned `acop.*`, `acop.mod/.csv`).
