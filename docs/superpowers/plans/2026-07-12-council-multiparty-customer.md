# Council Workflow v2 (Multi-Party Customer) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `council` workflow's single composite `customer` seat with four buy-side seats carrying encoded standing, so an unresolved veto cannot be outvoted by enthusiasm.

**Architecture:** The council is a single self-contained JavaScript file executed by the Workflow tool. It runs three rounds — blind parallel positions, full-barrier cross-examination, chair synthesis. This plan moves the file into the repo (it previously lived only as a session artifact, which is why a prior revision silently never landed), grows the `business` roster from 5 seats to 8, adds a `standing` field that is printed into the transcript the chair reasons over, and adds `blockers` + a `blocked` consensus state to the verdict schema.

**Tech Stack:** Plain JavaScript (not TypeScript — type annotations fail to parse). Claude Code Workflow runtime: `agent()`, `parallel()`, `phase()`, `log()`, `args`. No filesystem, no Node APIs, and `Date.now()` / `Math.random()` / argless `new Date()` throw.

## Global Constraints

- **File is self-contained.** No imports, no requires. The Workflow tool takes one script.
- **`meta` must be a pure literal** — no variables, function calls, spreads, or template interpolation. Required fields `name` and `description`.
- **`meta.phases` titles must match `phase()` call strings exactly.** Current titles: `Deliberate`, `Cross-examine`, `Chair`.
- **Plain JS only.** No `: string[]`, no interfaces, no generics.
- **Canonical repo root is `/home/dbreeden/Documents/development/shairozan/janus`.** The old `/home/dbreeden/GolandProjects/janus` path is the pharmalytica-era checkout and must not appear in the final file.
- **The `engineering` roster is not to be touched.** Neither are the lenses or research agendas of `devil's-advocate`, `champion`, `market`, or `strategist`.
- Source script to adapt (read-only reference): `/home/dbreeden/.claude/projects/-home-dbreeden-GolandProjects-janus/66beb3e7-24e3-4210-b6c4-3f85c47f1563/workflows/scripts/council-wf_4d6d801a-c7e.js`
- Branch: `council-multiparty-customer`. Spec: `docs/superpowers/specs/2026-07-12-council-multiparty-customer-design.md`.

---

## File Structure

| File | Responsibility |
|---|---|
| `.claude/workflows/council.js` | **Create.** The entire workflow: rosters, schemas, brief, three rounds. Single file by necessity — the Workflow runtime executes one self-contained script. |

One file, because the runtime gives no other option: it executes a single self-contained script, so the roster, schemas, and rounds cannot be split apart.

**On testing:** there is no unit-test layer here, and inventing one would be theater. A Workflow script can't be imported (it depends on runtime globals `agent()`, `parallel()`, `phase()`), and this is a Go repo with no JS test harness. Verification is therefore: `node --check` for syntax, `grep` assertions on the roster invariants that actually matter (13 seats, exactly 2 vetoes, no surviving `customer` seat, no stale repo path), and a real smoke run whose returned verdict is inspected. That is the honest set.

---

## Task 1: Move the workflow into the repo, verbatim, with a parameterized repo path

The point of this task is to establish the canonical file *before* changing any behavior, so that the roster diff in Task 2 is reviewable in isolation. Copy the source script unchanged except for the path fix.

**Files:**
- Create: `.claude/workflows/council.js` (adapted copy of the source script)

**Interfaces:**
- Produces: a workflow invocable as `Workflow({name: 'council'})`, accepting `args` as either a question string or `{question, roster?, artifact?, readOnly?, seats?, repo?}`.

- [ ] **Step 1: Copy the source script into the repo**

```bash
mkdir -p /home/dbreeden/Documents/development/shairozan/janus/.claude/workflows
cp /home/dbreeden/.claude/projects/-home-dbreeden-GolandProjects-janus/66beb3e7-24e3-4210-b6c4-3f85c47f1563/workflows/scripts/council-wf_4d6d801a-c7e.js \
   /home/dbreeden/Documents/development/shairozan/janus/.claude/workflows/council.js
```

- [ ] **Step 2: Add a `REPO` input, defaulting to the canonical root**

In the input-parsing block near the top, after the `READ_ONLY` line, add `REPO`:

```javascript
const input = typeof args === 'string' ? { question: args } : (args || {})
const QUESTION = input.question
const ROSTER = input.roster || 'engineering'
const ARTIFACT = input.artifact || 'the janus repository at its current HEAD'
const READ_ONLY = input.readOnly !== false // councils are read-only unless told otherwise
const REPO = input.repo || '/home/dbreeden/Documents/development/shairozan/janus'
```

- [ ] **Step 3: Interpolate `REPO` into the brief instead of hardcoding the old path**

In the `brief` template literal, the "Ground rules" section currently opens with a hardcoded path. Replace that line:

```javascript
- Repository: ${REPO} — "Janus", a Go + fyne.io desktop tool for managing NONMEM jobs on compute grids (SLURM/SGE/TORQUE), positioned as a cost-effective replacement for Certara Pirana, with CFR 21 Part 11 compliance and IQ/OQ validation as first-class concerns. There is also a Next.js customer portal in web/portal/.
```

- [ ] **Step 4: Verify the old path is gone and the file parses**

```bash
cd /home/dbreeden/Documents/development/shairozan/janus
grep -c GolandProjects .claude/workflows/council.js
node --check .claude/workflows/council.js && echo "PARSE OK"
```

Expected: `grep -c` prints `0`. `node --check` prints `PARSE OK`.

Note: `node --check` will parse the file despite `export const meta` because `.js` with an `export` is treated as an ES module by `--check`. If it complains about `export`, rename the check to `node --input-type=module --check < .claude/workflows/council.js`.

- [ ] **Step 5: Commit**

```bash
git add .claude/workflows/council.js
git commit -m "refactor: move council workflow into repo, parameterize repo path

The script previously existed only as a session artifact, which is why a
prior revision produced two byte-identical copies — there was no canonical
file to edit. No behavior change; the roster is untouched."
```

---

## Task 2: Split the composite `customer` seat into four buy-side seats with standing

**Files:**
- Modify: `.claude/workflows/council.js` — the `BUSINESS` array, and the `ENGINEERING` array (to add `standing: 'advisory'` to its five seats so the field is uniform).

**Interfaces:**
- Consumes: the `BUSINESS` array from Task 1.
- Produces: every seat object now carries `standing`, one of the exact strings `'veto'`, `'demand'`, `'signature'`, `'advisory'`. Downstream tasks read `s.standing` and `p.standing`.

- [ ] **Step 1: Add `standing: 'advisory'` to the four surviving business seats and all five engineering seats**

Each of `devil's-advocate`, `champion`, `market`, `strategist` (in `BUSINESS`) and `architect`, `implementer`, `red-team`, `contrarian`, `validation` (in `ENGINEERING`) gains one line alongside `seat`/`model`/`effort`. Their `lens` and `research` strings are **unchanged**. Example, for `champion`:

```javascript
  {
    seat: 'champion',
    model: 'opus',
    effort: 'high',
    standing: 'advisory',
    lens: `...unchanged...`,
    research: `...unchanged...`,
  },
```

- [ ] **Step 2: Delete the `customer` seat**

Remove the entire `customer` object from `BUSINESS` — the seat whose lens begins `You ARE the user: a pharmacometrician at a mid-size sponsor or CRO`, together with its `research` agenda.

- [ ] **Step 3: Add the four buy-side seats to `BUSINESS`**

Append these after `strategist`:

```javascript
  // ---- The buy side -------------------------------------------------------
  // These four are NOT one person. The one who wants it cannot approve it, and
  // the two who can kill it never open it. Modeling them as a single "customer"
  // averages away the conflict that actually decides the purchase.
  //
  // They evaluate Janus COLD — as an unknown vendor's tool that landed on their
  // desk. They are not told it is ours. A real QA reviewer has no loyalty.
  {
    seat: 'modeler',
    model: 'sonnet',
    effort: 'high',
    standing: 'demand',
    lens: `You are a pharmacometrician at a mid-size sponsor or CRO. You run NONMEM jobs on a grid, and your
reputation rides on results that survive audit. Some vendor's tool has landed on your desk and someone has
asked whether you would use it. Speak in the first person, from that chair.

You have NO purchasing power. You cannot approve this, you cannot sign for it, and nobody asked your
permission before buying the last tool you were made to use. What you have is demand: if you want this
badly, you can create pressure; if you shrug, it dies quietly no matter how good it is. Judge it on that
axis and be honest about which it is.

Walk the actual journey — submit, watch, cancel, browse history, generate IQ/OQ — and say what it feels
like. Where does it save you real time, and where does it merely relocate your pain? What would make you
refuse: losing a workflow you depend on, having no one to call when it breaks mid-submission, having to
learn a new tool for no gain? What would make you a zealot? Be concrete about the daily texture of the
work. Do not evaluate this from above; you have to LIVE with it.`,
    research: `Do not invent this persona from imagination — go LEARN it. Research how pharmacometricians
actually work day to day: NMusers threads, PAGE/ACoP abstracts and talks, pharmacometrics blogs, PsN and
Pirana documentation and tutorials, r/pharmacometrics, public accounts of NONMEM grid workflows. Find out
what tools they really run, what their gripes really are, and what a day actually looks like. Then speak as
that person, informed by what you found, and cite where you learned it. A persona built on vibes is
worthless to this council.`,
  },
  {
    seat: 'qa-csv',
    model: 'opus',
    effort: 'high',
    standing: 'veto',
    lens: `You are the computer system validation (CSV) reviewer in Quality Assurance. You will never open
this tool. You do not care whether it is pleasant to use. You care about exactly one thing: whether the
company can defend it to an inspector.

YOU HOLD A VETO. Your "no" ends this — no amount of enthusiasm downstairs overrides it. Understand also
that your "yes" is nearly worthless: clearing validation does not make anyone buy anything. So do not
soften. Your job is to find the reason this cannot be used for regulated work, and to state it plainly.

Interrogate: the IQ/OQ package (who wrote it, who qualified the writer, is a vendor's self-issued package
even acceptable?); the audit trail (is it attributable, legible, contemporaneous, original, accurate — and
is it tamper-evident?); the CFR 21 Part 11 claims specifically, which are easy to assert and hard to
survive; the self-validating state; whether the execution record that cohabitates with the model is
trustworthy evidence or merely a log file. Ask what happens when a run fails halfway. Ask who is
accountable when the tool is wrong.

State explicitly whether you BLOCK, and if so, exactly what would have to exist for you to withdraw the
block. "It would need X" is the most useful sentence you can produce.`,
    research: `You must not answer from memory; validation expectations move. Research what a QA/CSV review
actually demands before a modeler may use a tool for regulatory work. Look up: GAMP 5 software categories
and which one a tool like this falls into (that classification determines the entire validation burden);
the FDA's Computer Software Assurance (CSA) guidance and how it changed the calculus versus traditional CSV;
CFR 21 Part 11 and Annex 11 requirements for audit trails and electronic records; ISPE/GAMP material on
supplier assessment. Find out whether a small vendor's self-issued IQ/OQ documentation is accepted in
practice or whether the sponsor must re-validate in their own environment — that single question may decide
this council. Cite URLs. Report what you could not determine.`,
  },
  {
    seat: 'budget-holder',
    model: 'sonnet',
    effort: 'high',
    standing: 'signature',
    lens: `You are the head of pharmacometrics, or the IT director who owns this line item. You pay for it.
You will never touch it.

YOU HOLD THE SIGNATURE — but only downstream of the vetoes. You can say yes; you cannot overrule QA or
procurement, and you know better than to try. What you actually control is whether this is worth the
disruption at all.

Ask the questions a budget holder asks: what am I paying Certara today, and is that a renewal I could
credibly threaten or walk away from? What is the switching cost in people's time, in revalidation, in
retraining, in the six months where half the group is on each tool? What is my exposure if I adopt a tool
with no vendor behind it and it stops being maintained — do I now own a Go codebase nobody on my team can
read? Is the saving large enough to be worth ANY of this, or is it a rounding error against one FTE?

A cheaper tool that saves me 3% of a budget line and costs me a quarter of organizational churn is a bad
trade, and I will say so.`,
    research: `Research the money, and do not guess at it. Find what the incumbent stack actually costs and
how it is sold (per-seat? site licence? bundled into a platform deal?) — and if the pricing is opaque, say
so, because opacity is itself a finding about how this market sells. Find what switching between scientific
toolchains actually costs organizations in practice: revalidation effort, retraining, parallel-running
periods. Look for real accounts of migrations in regulated science. Find out what happens to organizations
that adopted an unmaintained open tool for regulated work. Cite URLs; a fabricated price poisons this
council.`,
  },
  {
    seat: 'procurement',
    model: 'sonnet',
    effort: 'high',
    standing: 'veto',
    lens: `You are vendor qualification / procurement. You do not care what this software does. You have not
read its feature list and you will not. You care whether this SUPPLIER can be onboarded at all.

YOU HOLD A VETO, and you exercise it routinely without apology. Your "yes" means only that a purchase order
is now legally possible — it is not an endorsement and it does not make anyone want the thing.

Run the gate: Can this entity survive a supplier audit? What is its financial viability — can it show
accounts, or is it one person? Who carries professional indemnity and liability insurance? Who indemnifies
us if this tool contributes to a bad submission? Is there source-code escrow, and if the maintainer is hit
by a bus tomorrow, what exactly do we own? Who passes our security review, and who signs our data
processing terms? What is the support SLA, and who answers the phone at 2am during a submission window?
Has this supplier ever been qualified anywhere, by anyone?

Be blunt about the bus factor. A single-maintainer supplier is not a risk to be managed, in my
world — it is usually a disqualification, and the burden is on them to show me the structure that gets
around it. State whether you BLOCK and what specifically would clear it.`,
    research: `Research how a small vendor actually gets onto an approved-supplier list at a pharma sponsor
or a CRO, and how long it takes — that sales-cycle length may be the most important number in this entire
council. Look up: supplier qualification and vendor audit requirements in GxP environments, what
documentation a supplier must produce, typical insurance and indemnification demands, whether source-code
escrow is standard. Find out whether large regulated buyers will purchase from a one-person company AT ALL,
and what structures — reseller, CRO partnership, consultancy-first, incorporation with real accounts — are
used to get around it. Find real accounts of small vendors succeeding or failing at this gate. Cite URLs.`,
  },
```

- [ ] **Step 4: Verify the roster shape**

```bash
cd /home/dbreeden/Documents/development/shairozan/janus
node --check .claude/workflows/council.js && echo "PARSE OK"
grep -c "seat: '" .claude/workflows/council.js
grep -c "standing: '" .claude/workflows/council.js
grep -n "standing: 'veto'" .claude/workflows/council.js
```

Expected: `PARSE OK`. Seat count `13` (5 engineering + 8 business). Standing count `13` — every seat has one. Exactly two `veto` lines, for `qa-csv` and `procurement`.

- [ ] **Step 5: Commit**

```bash
git add .claude/workflows/council.js
git commit -m "feat(council): split composite customer seat into four buy-side seats

modeler (demand), qa-csv (veto), budget-holder (signature), procurement
(veto). Each evaluates Janus cold, as an unknown vendor's tool. The seat
they replace was carrying four conflicting interests at once."
```

---

## Task 3: Surface standing in the transcript and add `blockers` to the verdict

Standing is worthless if the chair never sees it. This task puts it in the text the chair actually reads, and gives the verdict somewhere to record a block.

**Files:**
- Modify: `.claude/workflows/council.js` — the `VERDICT` schema, the `transcript` and `rebuttalText` builders, the chair prompt, and the returned object.

**Interfaces:**
- Consumes: `standing` on every seat, from Task 2.
- Produces: verdict objects with `blockers: [{seat, blocking, whatWouldUnblock}]` and `consensus` including `'blocked'`.

- [ ] **Step 1: Add `blockers` to the `VERDICT` schema and `blocked` to `consensus`**

```javascript
const VERDICT = {
  type: 'object',
  required: ['judgment', 'rationale', 'consensus', 'blockers', 'dissent', 'conditions', 'openQuestions'],
  properties: {
    judgment: { type: 'string', description: 'The council\'s answer, stated so someone could act on it tomorrow.' },
    rationale: { type: 'string', description: 'The reasoning that actually carried the room.' },
    consensus: { type: 'string', enum: ['unanimous', 'majority', 'split', 'no-consensus', 'blocked'] },
    blockers: {
      type: 'array',
      description: 'One entry for EVERY seat holding standing "veto", whether or not it blocked. A veto-holder who did not block must still appear, with blocking=false. If any entry has blocking=true and the block was not retired during cross-examination, consensus MUST be "blocked".',
      items: {
        type: 'object',
        required: ['seat', 'blocking', 'whatWouldUnblock'],
        properties: {
          seat: { type: 'string' },
          blocking: { type: 'boolean', description: 'Did this veto-holder block, and did the block survive cross-examination?' },
          whatWouldUnblock: { type: 'string', description: 'The specific thing that would have to exist for the block to be withdrawn. If blocking=false, say what kept them from blocking — that is load-bearing and may be fragile.' },
        },
      },
    },
    dissent: {
      type: 'array',
      description: 'Minority positions, preserved rather than averaged away. A council that erases its dissent is worth nothing.',
      items: {
        type: 'object',
        required: ['seat', 'position', 'whyItMightBeRight'],
        properties: {
          seat: { type: 'string' },
          position: { type: 'string' },
          whyItMightBeRight: { type: 'string' },
        },
      },
    },
    conditions: {
      type: 'array',
      description: 'What must hold for the decision to stay correct — the tripwires that should make us revisit it.',
      items: { type: 'string' },
    },
    openQuestions: {
      type: 'array',
      description: 'What the council could not settle and a human must.',
      items: { type: 'string' },
    },
  },
}
```

- [ ] **Step 2: Print standing into the opening-positions transcript**

The `transcript` builder currently prints seat and confidence. Add standing, and gloss what it means so the chair cannot misread it:

```javascript
const STANDING_GLOSS = {
  veto: 'VETO — this seat\'s "no" is dispositive; its "yes" means little',
  demand: 'DEMAND — cannot approve; indifference kills quietly',
  signature: 'SIGNATURE — can approve, but only if no veto-holder blocks',
  advisory: 'ADVISORY — argues; holds no power over the purchase',
}

const transcript = positions
  .map((p) => `### Seat: ${p.seat} (standing: ${STANDING_GLOSS[p.standing] || p.standing}; confidence: ${p.confidence})
**Position:** ${p.position}
**Reasoning:** ${p.reasoning}
**Evidence:** ${p.evidence.join('; ') || '(none cited)'}
**Risks it accepts:** ${p.risks.join('; ') || '(none named)'}`)
  .join('\n\n')
```

- [ ] **Step 3: Carry standing through the rebuttals**

The rebuttal `.then()` currently returns `{ seat: p.seat, ...r }`. It must carry standing:

```javascript
  ).then((r) => (r ? { seat: p.seat, standing: p.standing, ...r } : null)),
```

And `rebuttalText` prints it:

```javascript
const rebuttalText = rebuttals
  .map((r) => `### Seat: ${r.seat} (standing: ${STANDING_GLOSS[r.standing] || r.standing}; confidence after cross-examination: ${r.confidence})
**Conceded:** ${r.concessions.join('; ') || '(nothing)'}
**Objected:** ${r.objections.join('; ') || '(nothing)'}
**Revised position:** ${r.revisedRecommendation}`)
  .join('\n\n')
```

- [ ] **Step 4: Instruct the chair on standing**

Insert this into the chair prompt, immediately after the existing "Weigh the arguments, do not count the votes" paragraph:

```
## Standing is not the same as persuasiveness

The seats do not have equal power, and the record above tells you which is which. Read it.

The count of positive seats is NOT the judgment. This council can and should return "seven of eight seats
liked it, and the answer is still no." That is not a failure of the council; it is the council working.

- A seat with VETO standing that blocks, and whose block was not retired during cross-examination, means the
  consensus is **blocked** — no matter how strong the champion was, no matter how many seats were positive.
  Do not average a veto into a dissent bullet. Do not let a majority outvote it. It does not work that way in
  the world you are modeling.
- A veto-holder's *approval*, by contrast, is nearly worthless as a buy signal. Passing validation or vendor
  qualification does not make anyone want the thing. Do not read it as enthusiasm.
- A DEMAND seat's enthusiasm cannot close anything. Its indifference, however, is fatal — quietly. Weigh it
  as a necessary condition, never a sufficient one.
- A SIGNATURE seat can say yes only downstream of the vetoes.

You MUST fill `blockers` with one entry for EVERY veto-holding seat, including those that did not block.

If a veto was RETIRED during cross-examination — the veto-holder conceded that some specific thing genuinely
satisfies them — say so explicitly and loudly in your rationale. That is the single most informative event
this council can produce, and it must not be buried as a concession bullet.
```

- [ ] **Step 5: Return standing in the summary object**

```javascript
return {
  question: QUESTION,
  seats: positions.map((p) => ({ seat: p.seat, model: p.model, standing: p.standing, opening: p.position, confidence: p.confidence })),
  rebuttals,
  verdict,
}
```

- [ ] **Step 6: Verify**

```bash
cd /home/dbreeden/Documents/development/shairozan/janus
node --check .claude/workflows/council.js && echo "PARSE OK"
grep -c "blocked" .claude/workflows/council.js
grep -n "STANDING_GLOSS" .claude/workflows/council.js
```

Expected: `PARSE OK`. `STANDING_GLOSS` defined once and referenced in both `transcript` and `rebuttalText` (3 hits). `blocked` appears in the consensus enum and in the chair prompt.

- [ ] **Step 7: Commit**

```bash
git add .claude/workflows/council.js
git commit -m "feat(council): encode veto standing in transcript and verdict

Standing is printed into the text the chair reasons over, not merely
asserted in a prompt. VERDICT gains blockers[] and a 'blocked' consensus
state, so an unresolved veto cannot be outvoted by a positive majority."
```

---

## Task 4: Smoke-run the council and inspect the verdict

The only real end-to-end verification. This spends agents — 17 of them, several on opus — so it is a deliberate, gated step, not something to run casually.

**Files:** none modified.

- [ ] **Step 1: Run the business roster against a question that should provoke the veto seats**

```
Workflow({
  name: 'council',
  args: {
    question: 'Should Janus be sold as a commercial, validated replacement for Certara Pirana to pharma sponsors and CROs — or is that market structurally closed to a solo-maintainer vendor?',
    roster: 'business',
  },
})
```

- [ ] **Step 2: Check the returned object against the spec's success criteria**

Confirm, in the returned `verdict`:

1. `blockers` has an entry for **both** `qa-csv` and `procurement` — present even if `blocking: false`.
2. If either has `blocking: true` and the block was not retired, `consensus` is `'blocked'`, **even if most seats were positive**. If a majority were positive and consensus is not `blocked` despite a live veto, the chair prompt failed and Task 3 Step 4 needs to be strengthened.
3. `whatWouldUnblock` is concrete enough to act on — a thing that could be built or bought, not "improve compliance."
4. The `modeler`'s enthusiasm is not being read as a purchase signal in the rationale.

- [ ] **Step 3: Record the outcome**

If the council returns `blocked`, that is a real finding about the business, not a bug in the workflow. Report it as such. Do not tune the roster until it produces a comfortable answer — that would rebuild the exact defect this plan exists to remove.

---

## Self-Review

**Spec coverage.** Location → Task 1. Repo path → Task 1. Roster 5→8 with cold framing → Task 2. Standing encoded and printed → Tasks 2, 3. `blockers` + `blocked` → Task 3. Chair veto instruction → Task 3 Step 4. Engineering roster untouched → constrained globally, Task 2 Step 1 only adds `standing: 'advisory'`. Success criteria → Task 4 Step 2. No gaps.

**Placeholders.** None. Every code step carries the actual content.

**Type consistency.** `standing` is the same field name in `ENGINEERING`, `BUSINESS`, `STANDING_GLOSS`, the position `.then()` spread (`{...s, ...p}` carries it from the seat), the rebuttal `.then()` (explicitly re-added, since `REBUTTAL` does not return it), the two transcript builders, and the returned `seats` array. The four standing values are the identical string set in the seats, `STANDING_GLOSS`, and the check script.

One live risk worth naming: the rebuttal round's `.then()` must explicitly re-attach `standing` (Task 3 Step 3), because unlike round 1 it does **not** spread the seat object. Dropping that line yields `standing: undefined` in the chair's rebuttal transcript, and the veto silently stops being visible at exactly the moment it matters — no grep catches it, because `STANDING_GLOSS` is still present and still referenced. Verify Task 3 Step 3 by eye.
