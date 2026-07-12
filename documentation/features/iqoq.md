# Feature: User-Launchable IQ/OQ Qualification (GUI + executor)

## Context

Janus today has a build-tagged **CI** validation suite (`internal/validation`, `//go:build
validation`, 52 CFR-21 REQs) — but no way for an *end user* to qualify their own install at runtime.
Pharma users (and the operator running demos) need to answer two questions on their own machine:

1. **IQ — "Is Janus itself installed and operable?"** A lightweight self-check of the tool. No NONMEM.
2. **OQ — "Is my configuration valid, and is it functional?"** Two phases:
   - **Phase 1 (validity):** the *given configuration* is coherent — NONMEM pathing, the license, and
     mode-specific dependencies (Hermes image, docker socket, grid submit binary) are present.
   - **Phase 2 (functional):** if phase 1 passes, actually run an embedded open-source **ACOP** model
     through the user's configured execution path and verify the objective function value (OFV)
     matches a known reference within tolerance.

Both must be launchable from the GUI **Settings** ("Request run" + "last run" timestamp/status) and
headlessly from the **executor** binary via a switch. Each run writes a JSON report into
`~/.config/janus/qa/<uuid>/` (`iq.json` / `oq.json`) — a per-run, UUID-keyed audit trail.

The crypto/run-log machinery, NONMEM execution, OFV parsing, config, and an embeddable ACOP model all
already exist — this feature is a thin orchestration layer plus UI/CLI wiring.

---

## Design overview

A new **`internal/qa`** package holds all runtime qualification logic, **dependency-injected** so the
exact same core is callable from the GUI, the executor, and the cobra CLI (respects the repo's
orthogonality rule — build deps at the top layer, hand them down; `internal/qa` never reads viper,
never calls `os.UserHomeDir`, never constructs an executor).

A thin **`internal/qarun`** package sits *between* the callers and `internal/qa`: it builds the
`qa.ExecutorProvider` and executor probe from the real `execution` factory + `*config.Config`, so the
three callers share one wiring implementation and `internal/qa` stays pure. (`internal/qa` imports
`execution` only for the `Executor` interface type; it never constructs one — `qarun` does.)

```
caller (GUI / executor / cobra)          internal/qa (pure core)
  ├─ load *config.Config             ┐
  ├─ qarun.BuildExecutorProvider(cfg)├──▶  RunIQ(cfg, runDir, opts) -> *IQResult
  │    -> qa.ExecutorProvider            RunOQ(ctx, cfg, provider, summarizer, runDir, opts) -> (*OQResult, error)
  │    (same factory as real runs)      Store{ NewRunDir(), Save(dir,kind,v), Latest(kind) }
  ├─ qarun.NewExecutorProbe()        ┘    (atomic write, UUIDv7 — mirrors internal/runlog/store.go)
  ├─ Store.NewRunDir() -> uuid,dir
  └─ Store.Save(dir, "iq"|"oq", res)
```

**`RunIQ` returns `*IQResult` (no error)** — a failed self-check is data (recorded as a failing
`Check`), not a transport error.

**OQ takes a `qa.ExecutorProvider func(modelPath) (execution.Executor, error)`, not a pre-built
executor.** This is the one deviation from the original sketch: the Hermes executor must be built
*against the model path*, and OQ materializes the embedded ACOP model into the run dir internally — so
the executor can only be built after that. The provider lets `qarun` build local **or** Hermes lazily
(for Hermes it also generates a `.janus.config.json` from `cfg.Hermes` next to the model). It still
"uses the same execution factory the GUI uses for real runs," validating *whatever mode the user
configured*. Grid (async) functional runs remain out of scope for v1 — phase 1 still validates grid
config and phase 2 records a skip.

---

## IQ — tool self-check (no NONMEM)

`RunIQ(cfg *config.Config, runDir string, opts Options) *IQResult` — ordered checks, each a
`Check{Name, Status(pass|fail|skip), Reason, Category}`; overall = fail if any required check fails.
The config dir checked by `config_dir_writable` is derived as the grandparent of `runDir`
(`qa/<uuid>`'s parent's parent), so `internal/qa` never calls `os.UserHomeDir`. The
`executor_component` check runs `opts.ExecutorProbe` (provided by `qarun.NewExecutorProbe`, which runs
the bundled `executor --executor-version`); a nil probe is a skip, not a fail:

| Check | Logic |
|---|---|
| `janus_build_info` | `cfg.Version` populated (build ldflags present) — report version/commit |
| `config_loads` | config parsed; the caller hands in the loaded `*config.Config` (or a load error) |
| `config_dir_writable` | `~/.config/janus` exists & writable (temp-file probe) |
| `qa_dir_writable` | `runDir` (under `~/.config/janus/qa/<uuid>`) writable |
| `executor_component` | the bundled executor is locatable/runnable (`--executor-version` exits 0) — Janus delegates execution to it, so it's part of "the tool" |

No NONMEM, license, Hermes, or docker probing here — those are OQ phase 1.

---

## OQ — phase 1 (config validity) + phase 2 (functional)

`RunOQ(ctx, cfg, provider qa.ExecutorProvider, summarizer, runDir, opts) (*OQResult, error)`

**Phase 1 — validate the given configuration** (checks, fail-fast: skip phase 2 if any required fail).
`cfg` embeds `Input`, so fields are reached directly (`cfg.NonmemPath`, `cfg.ExecutionMode`, …):
- `nonmem_binary` — `filepath.Join(cfg.NonmemPath, cfg.NonmemBinary)` resolves, exists, executable
  (`runtime.GOOS`-gated exec bit). **Skipped** for `HERMES` (NONMEM lives in the container) and grid
  (NONMEM lives on the compute node).
- `nonmem_license` — via the existing `config.ValidateNONMEMLicenseForExecution(cfg)` fallback chain
  (`nonmem.license.path` → `~/nonmem.lic` → `./nonmem.lic`, `config.go:381-415`); gated by
  `config.RequiresNONMEMLicense(mode)`.
- **mode-specific deps** (driven by `cfg.ExecutionMode` / `cfg.Scheduler`):
  - `HERMES` → `docker_socket` reachable (`/var/run/docker.sock` stat) + `hermes_image` (`cfg.Hermes.Image`) set.
  - grid (`Scheduler != ""`) → `grid_submit_binary` (`sbatch` for SLURM, `qsub` for SGE/TORQUE/PBS) on `PATH`.
- Result records `Phase: "preflight"` on failure with the offending check + reason.

**Phase 2 — functional run** (only if phase 1 passes; skipped with a note for grid):
1. Materialize embedded `acop.mod` + `acop.csv` into `runDir` (data sits next to the model — `$DATA acop.csv`).
2. Build the executor: `provider(runDir/acop.mod)` (local `CreateExecutor(mode)` or, for Hermes, generate
   `.janus.config.json` then `CreateHermesExecutor`). A build failure records a `executor_build` fail check.
3. `executor.Execute(ctx, runDir/acop.mod, false, 1, false, nil)`. Working dir = `runDir`; `.lst` lands there.
4. Persist artifacts in `runDir` for the audit trail (model, data, `.lst`/`.ext`, `stdout.txt`/`stderr.txt`).
5. Parse OFV: `summarizer.SummarizeModel(...)` → `ModelSummary.GoodnessOfFit.ObjectiveFunctionValue *float64`.
6. **Pass gate (four checks, all must hold):** `nonmem_execution` (`err==nil && ExitCode==0 && .lst exists`)
   **AND** `ofv_present` (present & finite) **AND** `ofv_within_tolerance` of the reference **AND**
   `minimization_successful` (`EstimationSummary.Minimized` — confirmed available, wired as the 4th gate).

**OQ tolerance (match reference OFV ± tolerance).** `internal/qa/reference.go`:
```go
const ReferenceOFV = 2636.8457689964607 // from testdata/mock-nonmem/acop.lst
const AbsTolerance = 1.0
const RelTolerance = 1e-3 // 0.1% ≈ ±2.6 OFV units — absorbs cross-version trailing-digit drift,
                          // still catches a genuinely wrong/broken result.
```
Pass if `abs(got-ref) <= AbsTolerance` OR `abs(got-ref)/abs(ref) <= RelTolerance`.

> **Note on observed precision:** the summarizer parses the *reported* `#OBJV:` line from the `.lst`
> (`2636.846`), which is rounded relative to the full-precision `ReferenceOFV`
> (`2636.8457689964607`, the "OBJECTIVE FUNCTION VALUE WITHOUT CONSTANT" line). They differ by
> ~2.3e-4 — well inside `AbsTolerance`. This is exactly what the tolerance band exists to absorb.
>
> **Confirmed during build:** `summary` *does* expose a minimization-success flag
> (`model.EstimationSummary.Minimized`), so it is wired in as the 4th OQ gate above.

---

## QA store + output (new `qa/` dir only)

`internal/qa/store.go` — mirrors `internal/runlog/store.go` (atomic `.tmp`+rename, 0600, UUIDv7 via
`uuid.Must(uuid.NewV7())`):
- `NewRunDir() (id, dir, err)` — UUIDv7 + `mkdir ~/.config/janus/qa/<uuid>/`.
- `Save(dir, kind, v)` — atomic write `<dir>/<kind>.json` (`kind` = `iq`|`oq`).
- `Latest(kind) (path, raw, err)` — newest by UUIDv7 ordering (time-sortable); `os.ErrNotExist` sentinel = "never run".
- No `index.json` (private per-user dir, `Latest` is a cheap directory sort — YAGNI).

The pre-existing `config.Validation.IQ/OQ` single-file path fields are **left unused** (mark deprecated
in a comment); the UUID dir is the sole source of truth. Do not remove the fields (persisted config).

Report types (`types.go`) mirror the validation-suite JSON shape (name/status/reason/category +
timestamps); `OQResult` additionally carries `Phase`, `NonmemExitCode`, `ObservedOFV`, `ReferenceOFV`,
`Abs/RelTolerance`, `Artifacts[]`. Provenance (`JanusVersion`, `User`) from `cfg.Version`/`cfg.User`.

---

## GUI wiring (`internal/gui/settings.go` + `app.go`)

A dedicated, delineated **"Runtime Qualification (IQ/OQ)"** section (separators + bold header + caption)
below the legacy "Validation (CFR 21 Part 11)" path entries:
- Buttons **"Run IQ"**, **"Run OQ"**; labels **"Last IQ: <ts> — PASS/FAIL"**, **"Last OQ: …"** (populated on open from `store.Latest`).
- **"Download IQ" / "Download OQ"** buttons (`theme.DownloadIcon()`) save the latest `iq.json`/`oq.json` via a `dialog.NewFileSave` (default name `janus-<kind>-report.json`) so users don't hunt the filesystem. Each is **disabled until a report of that kind exists** (`refreshDownloadButtons`, re-evaluated after every run).
- On tap: disable button → toast (`app.go:2741`) → **background goroutine** (pattern `app.go:3500-3546`, `context.WithTimeout`): `store.NewRunDir()` → `qa.RunIQ`/`RunOQ` → `store.Save` → **`fyne.Do(...)`** (`app.go:3124`) to re-enable, update the last-run label, toast success/`sendError`.
- OQ builds the executor via `qarun.BuildExecutorProvider(a.config)` (same factory as real runs) + `summary.NewNONMEMSummarizer(...)`, handed into `qa.RunOQ`. The OQ button is disabled during its (longer) run, wrapped in `context.WithTimeout(a.errorCtx, 30*time.Minute)`.
- The QA base dir comes from `qarun.DefaultQABaseDir()` (`~/.config/janus/qa`, same logic as `getDefaultConfigPath()`), shared with the executor and cobra callers rather than a GUI-private helper.

---

## Executor + CLI wiring

- **Executor switch (implemented):** `RunIQ`/`RunOQ` bools on `ExecutorFlags` (`cmd/executor/args.go`) parsed as `--executor-run-iq` / `--executor-run-oq`; `main.go` branches into `runQualification` (after `--executor-version`/`--executor-help`, before the Hermes flow) which loads the full config via the existing `loadJanusConfig`, builds DI deps via `qarun`, calls `qa.Run*`, prints a summary (honors `--executor-quiet`), and **exits 0 on pass / non-zero on fail**.
- **Cobra subcommand (implemented):** `cmd/janus/commands/validate/` (`janus validate iq` / `validate oq`) via the functional factory pattern, registered in `cmd/root.go`. Config loads through `config.NewInitializer`; output goes to `cmd.OutOrStdout()` (avoids the `forbidigo` ban on `fmt.Print*`); a failing qualification returns an error so the command exits non-zero (`SilenceUsage` keeps it quiet).

All three callers are thin: `qarun` builds deps at the top, they call `qa.Run*`, format, set exit code. Zero logic duplication.

---

## Package / file layout

```
internal/qa/                        # pure core (never constructs an executor)
  types.go       # Status, Check, Kind, Options, Summarizer iface, ExecutorProvider, IQResult, OQResult
  store.go       # Store: NewRunDir / Save / Latest (port from runlog/store.go)
  iq.go          # RunIQ + tool-self checks
  oq.go          # RunOQ: phase-1 validity + phase-2 acop run + 4 gates; ExecutorProvider
  reference.go   # ReferenceOFV + tolerance consts
  embed.go       # //go:embed acopfiles/acop.mod acopfiles/acop.csv  (embed can't reach testdata/)
  acopfiles/     # byte-identical copies of testdata/mock-nonmem/acop.{mod,csv} (CI byte-equality guard)
  *_test.go
internal/qarun/                     # wiring: qa <-> execution factory (imports qa + execution + config)
  qarun.go       # BuildExecutorProvider(cfg), NewExecutorProbe(), LocateExecutor(), DefaultQABaseDir()
  oq_e2e_test.go # //go:build validation — full loop via testdata/mock-nonmem
cmd/executor/args.go               # +RunIQ/RunOQ flags
cmd/executor/qualification.go      # runQualification headless branch (main.go dispatches to it)
cmd/janus/commands/validate/...    # validate iq|oq subcommand
internal/gui/settings.go           # Settings buttons + last-run labels + goroutine/fyne.Do wiring
```

## Reuse points (confirmed)
- `internal/runlog/store.go` — atomic write + UUIDv7 + latest pattern to mirror.
- `internal/execution` — `Executor` interface (`interface.go:28`), the factory that builds the per-mode executor for real runs, `NONMEMExecutor.BuildCommand`/`Execute`.
- `internal/summary` — `NONMEMSummarizer.SummarizeModel` → `GoodnessOfFit.ObjectiveFunctionValue` (`model/summary.go:74`, OFV regex `nonmem_parser.go:91`).
- `internal/config/config.go` — `Input.{NonmemPath,NonmemBinary,NONMEM,ExecutionMode,Scheduler,Version,User}`, license fallback (`381-415`), config-dir helper (`563-572`).
- `testdata/mock-nonmem/{acop.mod,acop.csv,acop.lst}` — ACOP model + reference OFV; mock NONMEM script for hermetic E2E.
- `github.com/google/uuid` (UUIDv7) — already a dependency.

---

## Testing (hermetic — no real NONMEM)

Build the core (steps below) and get it green with fakes **before** any UI/CLI wiring.
- **IQ** (`iq_test.go`): `t.TempDir()` configs — missing/non-exec/exec binary paths, writable/non-writable dirs, build-info present/absent. Skip exec-bit cases on Windows.
- **OQ** (`oq_test.go`): a `fakeExecutor` implementing `execution.Executor` that copies `testdata/mock-nonmem/acop.lst` into the run dir and returns `ExitCode:0`, paired with the real summarizer (or a `fakeSummarizer` returning a chosen OFV). Cases: OFV at/within/outside band, nil OFV, ExitCode!=0, .lst missing, phase-1 fail → executor never called. Inject `Options.Now` for deterministic timestamps.
- **Store** (`store_test.go`): unique dirs, valid atomic JSON (no `.tmp` left), `Latest` ordering + not-found sentinel.
- **Embed integrity**: assert `internal/qa/acopfiles/acop.{mod,csv}` byte-equal the `testdata/mock-nonmem` fixtures.
- **Tagged E2E** (`internal/qarun/oq_e2e_test.go`, `//go:build validation`): run OQ through the real `testdata/mock-nonmem/mock-nonmem-fast.sh` as the NONMEM binary via the real `execution` factory (materialize→execute→parse loop). The `validation` tag means default `go test` stays hermetic and `mage docker:validation` picks it up automatically. Skipped on Windows (bash script).

## Implementation order
1. `types.go` → 2. `store.go` → 3. `embed.go`+`acopfiles/`+`reference.go` → 4. `iq.go` → 5. `oq.go`
→ 6. core tests (2–5 green hermetically) → 7. executor flags + headless branch → 8. cobra `validate`
→ 9. GUI Settings buttons + goroutine/`fyne.Do` + `qaBaseDir()`.

---

## Verification
- `mage docker:unit` (or `go test ./internal/qa/...`) — core green with fakes.
- Tagged mock E2E: OQ against the `testdata/mock-nonmem` script → asserts OFV within tolerance, `oq.json` written under a `qa/<uuid>/` dir with artifacts.
- Manual (GUI): Settings → Run IQ → PASS + timestamp; Run OQ on a NONMEM-less box → phase-1 fail with a clear reason; on a licensed box → phase-2 run, OFV matches reference, `~/.config/janus/qa/<uuid>/oq.json` + `.lst` present.
- Manual (executor): `executor --executor-run-iq` and `--executor-run-oq` exit 0/non-zero appropriately and write the same `qa/<uuid>/` reports.

## Decisions
- **Locked:** IQ = Janus tool self-check only; OQ phase-1 = config/dependency validity (NONMEM path, license, Hermes/docker/grid deps), phase-2 = run ACOP, **OFV must match reference ± tolerance**; output = `~/.config/janus/qa/<uuid>/` only (legacy `Validation.IQ/OQ` left unused/deprecated).
- **Resolved during build:**
  - Minimization-success flag **exists** (`model.EstimationSummary.Minimized`) → wired as the 4th OQ gate.
  - Tolerance band kept at the defaults (`AbsTolerance=1.0`, `RelTolerance=1e-3`); confirmed sufficient to absorb the reported-vs-full-precision OFV gap.
  - `janus validate` cobra subcommand **kept** (alongside the required executor switch).
  - **Executor parameter changed** from a pre-built `execution.Executor` to a `qa.ExecutorProvider func(modelPath) (execution.Executor, error)` so Hermes can be built against the materialized model; the wiring lives in the new `internal/qarun` package so `internal/qa` stays free of `execution`-construction.
  - `RunIQ` returns `*IQResult` (no error).
- **Scope note:** OQ phase-2 functional run covers local + Hermes execution modes (synchronous via the Executor); grid functional runs are future — phase-1 still validates grid config, phase-2 records a skip.