# Council workflow v2 — multi-party customer

**Date:** 2026-07-12
**Status:** Approved, not yet implemented

## Problem

The `council` workflow convenes a panel of seats that file independent positions,
cross-examine each other, and are then synthesized by a chair. Its `business` roster
models the customer as a **single composite seat**:

> "You ARE the user: a pharmacometrician at a mid-size sponsor or CRO who runs NONMEM
> jobs on a grid and whose reputation rides on results that survive audit."

That one seat is silently carrying four people with conflicting interests, and its own
lens admits it — the seat is asked to worry about daily UX, about defending the tool to
QA, and about having no one to call when it breaks, all at once.

Those are not one person's concerns, and collapsing them averages away the conflict that
actually decides a purchase in this market: **the person who loves the tool is not the
person who can say yes, and the person who can say no never touches it.**

A second, mechanical problem: the workflow was never durably stored. It existed only as a
session artifact under `~/.claude/projects/.../workflows/scripts/`. A previous attempt to
revise it produced two byte-identical copies — the script was re-invoked, never edited,
because there was no canonical file to edit.

## Goals

1. Model the customer as multiple distinct parties with **unequal power**.
2. Make the workflow durable and version-controlled.
3. Correct the stale hardcoded repository path.

## Non-goals

- No change to the `engineering` roster.
- No change to the three-round structure (positions → cross-examination → chair).
- No change to the four non-customer business seats' lenses or research agendas.

## Design

### Location and invocation

Move the script into the repository at `.claude/workflows/council.js`, invocable as
`Workflow({name: 'council'})`. This is the fix for the "revision that never happened":
a canonical file can be edited; a session artifact cannot.

### Repository path

Stop hardcoding the path. Accept `repo` in args, defaulting to
`/home/dbreeden/Documents/development/shairozan/janus`. The brief interpolates it.

The prior path, `/home/dbreeden/GolandProjects/janus`, is the pharmalytica-era checkout.
The code is the same; `shairozan/janus` is the canonical root going forward.

### Roster: 5 seats → 8 seats

The four existing seats — `devil's-advocate`, `champion`, `market`, `strategist` — keep
their lenses and research agendas verbatim. They were working.

The composite `customer` seat is deleted and replaced by four buy-side seats. Each
evaluates Janus **cold**: as an unknown vendor's tool that landed on their desk. They are
not told it is ours. A real QA reviewer or procurement officer has no loyalty and no
context, and the honest simulation is the harsh one.

| Seat | Model | Role | Standing |
|---|---|---|---|
| `modeler` | sonnet | Pharmacometrician. Runs the jobs, lives with the tool, signs nothing. | `demand` |
| `qa-csv` | opus | Computer system validation reviewer. Never opens the tool. Can block. | `veto` |
| `budget-holder` | sonnet | Head of pharmacometrics / IT director. Pays, doesn't touch it. | `signature` |
| `procurement` | sonnet | Vendor qualification. Doesn't care what it does; cares whether the vendor can be onboarded. | `veto` |

Research agendas, one per seat:

- **`modeler`** — build the persona from evidence, not imagination: NMusers threads,
  PAGE/ACoP abstracts, PsN and Pirana docs and tutorials, public accounts of NONMEM grid
  workflows. What do they actually run, what do they actually complain about.
- **`qa-csv`** — GAMP 5 software categories, CSV vs. CSA expectations, what a QA
  department demands before a modeler may use a tool for regulatory work, what an
  inspector asks for, whether a small vendor's self-issued IQ/OQ package is accepted.
- **`budget-holder`** — what the incumbent stack actually costs, what switching costs
  look like, what it means to own a tool with no vendor behind it.
- **`procurement`** — approved-supplier onboarding: financial viability checks, insurance,
  indemnification, source-code escrow, security review, bus factor. Whether a
  single-person company can be onboarded at all, and what structures (reseller, CRO
  partnership, consultancy-first) get around it.

### Standing is encoded, not merely described

Each seat carries a `standing` field: `veto` | `demand` | `signature` | `advisory`.
Non-buy-side seats are `advisory`.

Standing is **printed next to the seat in the cross-examination transcript and in the
chair's record**, so the asymmetry lives in the text the chair reasons over rather than in
a prompt instruction it may quietly fail to honor.

The power asymmetry being modeled:

- `veto` — a "no" is dispositive. A "yes" is worth almost nothing; passing vendor
  qualification does not make anyone buy you.
- `demand` — enthusiasm cannot close a deal, but indifference kills it quietly.
- `signature` — can say yes, but only if no veto-holder has said no.

### Verdict schema

`VERDICT` gains:

```
blockers: [{ seat, blocking: bool, whatWouldUnblock: string }]
```

`consensus` gains a `blocked` state alongside `unanimous | majority | split | no-consensus`.

The chair is instructed explicitly: **the count of positive seats is not the judgment.**
If a veto-holder blocks and the block is unresolved, the answer is `blocked` regardless of
how strong the champion was, and the useful output is `whatWouldUnblock` — not a score.

A veto **retired** during cross-examination (e.g. QA concedes the validation package does
satisfy them) must be recorded as retired. That is the single most informative event this
council can produce, and it must not be flattened into a concession bullet.

### Unchanged

Round 1 blind parallel positions; round 2 full-barrier cross-examination (all 8 read all
8, and fact-check each other's uncited external claims); round 3 chair synthesis with
dissent preserved.

## Cost

8 openings + 8 rebuttals + 1 chair = **17 agents**, up from 11, several on opus. A
business-roster run is materially more expensive than before. This is a deliberate trade:
the seats that were merged were merged because they were expensive, and merging them is
what broke the result.

## Success criteria

A business-roster run produces a verdict in which:

1. The QA/CSV and procurement seats each state plainly whether they block.
2. Any unresolved veto caps `consensus` at `blocked`, even when a majority of seats are
   positive.
3. `whatWouldUnblock` is concrete enough to act on.
4. The modeler's enthusiasm is visibly *not* treated as a purchase signal.
