# Proposal: Pirana Parity I — Feature Gap Analysis & Roadmap

## Context

We recently shipped a **Pirana configuration migrator** (`internal/pirana`, see
[`PIRANA_CONFIG_MIGRATION.md`](./PIRANA_CONFIG_MIGRATION.md)). Building it forced
us to read Pirana's full configuration surface, and the migrator already names the
gaps for us: every setting it reports as **"detected but not imported"** is a
Pirana capability Janus has no home for yet (see `unmappedKeys` in
`internal/pirana/mapping.go` — PsN, remote SSH/mount, R, pyDarwin, NLME, Stan).

This document turns that observation into a prioritized parity roadmap. For each
gap it states what Pirana does, what Janus does **today** (grounded in the current
code, not aspiration), what we propose, the config additions required, and — the
closing loop — what the **migrator** can then map.

### Guardrails (unchanged from project philosophy)

- **Validation scope holds.** We validate *system-call generation and output
  parsing*, not the external tool's math (per `CLAUDE.md`). Parity features add
  configuration → command generation → output parsing; they do not take on
  NONMEM/PsN/R correctness.
- **No new global state**; config flows top-down through layers; readers/executors
  stay in lower layers behind interfaces.
- **Migrator stays read-only** and only gains *more* fields to map as features land.

## Current parity snapshot

| Capability | Janus today | Completeness |
|-----------|-------------|--------------|
| Local NONMEM / BBI / Hermes execution | Implemented | ✅ high |
| SLURM grid (CLI + REST) | Implemented (`internal/slurm/`) | ✅ high |
| SGE / TORQUE / PBS grid | Enum values only; fall back to LOCAL | 🟡 ~5% |
| Remote execution over SSH | Absent | ❌ 0% |
| Configurable scheduler commands / polling regexes | Hardcoded for SLURM only | ❌ 0% |
| PsN execution | Stub — `BuildCommand` only, `Execute` returns "not implemented" | 🟡 ~10% |
| Per-model notes / coloring / OFV browser | Run-level data captured in run log; not surfaced per-model | 🟡 partial |
| Run-output dir policy (versioning / backup / cleanup) | Overwrites in place; `AutoBackup` field exists but is **unused** | ❌ 0% |
| Multiple NONMEM installations | Single `nonmem-path` / `nonmem-binary` | ❌ 0% |
| R / Stan / NMQual / WFN / NLME / pyDarwin | Only via generic Hermes container images | 🟡 ~20% |
| General prefs: run prefix, alt data dir, close-console | Absent | ❌ 0% |
| Settings validation UX (live, regex tester, profiles) | Basic on-save validation only | 🟡 ~50% |

## Feature proposals

Ordered by parity leverage. Effort is rough (S/M/L). "Migrator impact" notes which
`internal/pirana` needles graduate from `unmappedKeys` to real mappings.

### P0 — Make remote HPC actually work like Pirana

#### 1. Configurable workload managers (generalize beyond SLURM) — **L**
- **Pirana:** ships SGE / Slurm / Torque / jsub-Torque as *editable* profiles —
  submit command, additional args, run priority, job-name template
  (`{model}_{project}`), PsN params, script header (PBS directives), plus the poll
  commands (scheduled/running/finished/info/delete/get-nodes/get-queues) and the
  named-capture **polling regexes** for parsing `qstat`/`squeue` text.
- **Janus today:** SLURM commands are hardcoded in `internal/slurm/cli_client.go`;
  SGE/TORQUE/PBS are enum stubs that fall back to LOCAL.
- **Proposed:** externalize a scheduler into a `SchedulerProfile` (name, submit/
  cancel/poll/info commands, arg templates, job-name template, script header,
  polling regexes with named groups). Ship SLURM/SGE/Torque as built-in defaults;
  let users add/edit profiles. The SLURM CLI client becomes the first consumer of
  a generic `gridClient` driven by a profile.
- **Config additions:** `schedulers: []SchedulerProfile` (built-in defaults seeded).
- **Migrator impact:** maps Pirana's per-scheduler command + polling settings almost
  1:1 — graduates `scheduler`/`grid` and the polling/command keys.

#### 2. Remote execution over SSH + path mapping — **L**
- **Pirana:** connects to a cluster over SSH; maps **remote mount ↔ local mount**
  (e.g. `Z:` ↔ `/home/user/projects`) to translate paths before sending commands;
  Windows uses plink/OpenSSH (plink path on `PATH`).
- **Janus today:** no SSH anywhere; SLURM CLI runs `exec.CommandContext` locally,
  REST hits a local socket/URL. No path translation concept.
- **Proposed:** an SSH transport (`golang.org/x/crypto/ssh`) that runs the (now
  configurable, see #1) scheduler commands on a remote host, plus a path-mapper
  that rewrites local model paths to their remote equivalents before submit and
  back for result retrieval. Key-based auth only (no passwords stored).
- **Config additions:** `remote: { host, user, key_path, mounts: [{local, remote}] }`.
- **Migrator impact:** graduates the `remote` / `ssh` / `mount` needles currently in
  `unmappedKeys`.

#### 3. Run-output directory policy — **M**
- **Pirana:** "disallow overwriting" → sequential numbered dirs (`modelfit_dir1`,
  …) with latest copied to root; auto-backup to a `backup/` subfolder; auto-cleanup
  of intermediate runtime files after estimation.
- **Janus today:** outputs overwrite in place; `config.ProjectsConfig.AutoBackup`
  exists (`internal/config/config.go`) but is **never read**; no cleanup.
- **Proposed:** an output policy applied by the executors — overwrite-protect with
  sequential run dirs, optional backup copy, and post-run cleanup driven by retain/
  prune globs (Hermes already has a `Retain []string` concept to reuse).
- **Config additions:** `runs: { overwrite_policy, sequential_dirs, auto_backup,
  cleanup_globs }` (wire up the existing `AutoBackup`).
- **Migrator impact:** maps Pirana's general-prefs booleans (backup/cleanup/overwrite).

### P1 — Daily-workflow parity

#### 4. PsN integration depth — **M/L**
- **Pirana:** PsN execute dialog (simple/advanced), help strings, command history,
  pre/post R scripts, `psn.conf`, and presets for vpc / bootstrap / scm.
- **Janus today:** `internal/execution/psn.go` `Execute()` returns "not
  implemented"; only `BuildCommand` produces a string.
- **Proposed:** implement `Execute` over the generic grid/SSH transport from #1–2;
  add command presets (vpc/bootstrap/scm) with a recently-used history, and
  optional pre/post hook scripts. Honor an existing `psn.conf` rather than
  reimplementing it.
- **Config additions:** `psn: { path, conf_path, presets, pre_script, post_script }`.
- **Migrator impact:** graduates the `psn` needle.

#### 5. Per-model metadata & coloring (the `pirana.dir` story) — **L**
- **Pirana:** a cohabiting hidden DB per model directory storing notes, OFVs, run
  success, gradients, and **model coloring**, surfaced in a model overview.
- **Janus today:** the run log (`internal/runlog`, `RunRecord`) already captures
  OFV, parameter estimates, minimization status, run time, exit code, signatures —
  but there is no per-model browser and no notes/colors. `.janus.config.json`
  (`ModelConfig`) holds only execution config.
- **Proposed:** a model-list view that derives status/OFV/coloring from the
  **existing run log** (no new source of truth) and adds user notes/tags persisted
  in a cohabiting sidecar — consistent with the "log that cohabitates with the
  model" goal in `CLAUDE.md`. This is mostly a UI + small storage lift; the data
  largely exists.
- **Config additions:** minimal; mostly storage + UI.
- **Migrator impact:** optional — index existing `pirana.dir` notes/colors (Phase 4
  of the migration proposal).

#### 6. General run preferences — **S**
- **Pirana:** model/run filename prefix (also filters them from the overview),
  alternative data-file directory, "close console after run".
- **Janus today:** none of these in `config.Input` or the settings UI.
- **Proposed:** add the three preferences and wire them (prefix into run-dir/file
  naming and the eventual model list filter; alt data dir into data-file resolution;
  close-console into the GUI run panel).
- **Config additions:** `run_prefix`, `alt_data_directory`, `close_console_after_run`.
- **Migrator impact:** maps Pirana's "prefix for models/runs", "alternative
  data-file directory", and "close console window after run".

#### 7. Multiple NONMEM installations — **M**
- **Pirana:** manages several NONMEM installs and lets the user pick a default /
  switch per run.
- **Janus today:** a single `nonmem-path` + `nonmem-binary`.
- **Proposed:** an `installations: []NonmemInstall{ name, path, binary }` list with a
  default; execution options gain a version selector. Keep the single-field config
  working by treating it as the implicit default install (backward compatible).
- **Migrator impact:** import all detected NONMEM versions, not just one.

### P2 — Longer-horizon / specialized engines

8. **Software-integration hooks (R diagnostics, Stan, NMQual, WFN)** — **M.** A
   generic pre/post-run script registry plus named tool entries, so R diagnostics
   can run after a fit. Reuses the hook mechanism proposed in #4. Graduates the
   `rscript`/`stan` needles.
9. **RsNLME / NLME engine** — **L.** A new execution mode with its own config (MPI
   exec, GCC, NLME exec, license, `INSTALLDIR`). Large; only if there's user demand.
   Graduates the `nlme` needle.
10. **pyDarwin integration** — **L.** Python venv + `system_options.json` + interpreter
    path. Niche; large. Graduates the `darwin` needle.
11. **Settings validation UX & profiles** — **M.** Live field validation with
    required-field markers, named config **profiles**, portable mode, and — pairing
    naturally with #1 — a **regex test panel** for polling patterns (Pirana ships
    exactly this). Improves trust for the regulated-environment audience.

## Recommended "Parity I" scope (first wave)

Tackle the bundle that makes **remote HPC genuinely usable** and is highest-leverage
against configs the migrator already detects:

1. **#1 Configurable workload managers** (unlocks SGE/Torque + polling regexes)
2. **#2 Remote SSH + path mapping** (the actual cluster story)
3. **#3 Run-output directory policy** (safety/versioning users expect daily)
4. **#6 General run preferences** (cheap, immediately visible parity)

Each lands with its **migrator mapping** in the same change, shrinking
`unmappedKeys` so the import gets richer as parity grows. #4 (PsN) and #5 (model
browser) are the natural "Parity II" follow-on.

## Roadmap

| # | Feature | Priority | Effort | Migrator needle graduated |
|---|---------|----------|--------|----------------------------|
| 1 | Configurable workload managers | P0 | L | scheduler/grid, polling, commands |
| 2 | Remote SSH + path mapping | P0 | L | remote, ssh, mount |
| 3 | Run-output directory policy | P0 | M | backup, cleanup, overwrite |
| 6 | General run preferences | P1 | S | prefix, alt data dir, close-console |
| 4 | PsN integration depth | P1 | M/L | psn |
| 5 | Per-model metadata & coloring | P1 | L | (pirana.dir, optional) |
| 7 | Multiple NONMEM installations | P1 | M | nonmem versions |
| 8 | Software-integration hooks | P2 | M | rscript, stan |
| 9 | RsNLME / NLME engine | P2 | L | nlme |
| 10 | pyDarwin integration | P2 | L | darwin |
| 11 | Validation UX & profiles | P2 | M | — |

## Open questions

1. **Remote auth surface:** key-only (recommended) vs. agent/plink passthrough on
   Windows? How much of Pirana's plink-on-PATH behavior do we replicate?
2. **Scheduler profiles format:** ship built-in defaults editable in-app, and/or an
   import/export file (Pirana supports importing a workload-manager settings file)?
3. **Model browser scope:** is "Parity I" UI-light (settings + execution only), with
   the model overview deferred to Parity II?
4. **Engine breadth:** is NLME/pyDarwin in scope for this product at all, or do we
   explicitly cede those to keep focus on NONMEM/PsN + Hermes?
