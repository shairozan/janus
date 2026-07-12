# Janus

*One workbench. Two faces. Every modeling job as simple as choosing a container.*

---

## The short version

Janus is a desktop workbench for pharmacometric modeling that takes the hardest,
most fragile parts of running a model — the environment, the toolchain, the
"which version of which engine on whose machine" — and reduces them to a single
choice: **which container do you want to run in?**

Switching your modeling engine from one version to the next isn't a weekend of
reinstalls and a careful note in the validation binder. It's picking a different
container from a list. The job runs the same way on your laptop, on the grid, or
in an isolated sandbox — and Janus keeps the receipts.

Named for the Roman god of thresholds — the one who looks two ways at once —
Janus was built for two audiences who rarely get served by the same tool. The
modeler who just wants the job to run. The IT and QA professionals who need to
prove exactly *how* it ran. Both faces, one product.

---

## The idea at the center: the environment is a choice, not a chore

For most teams, the modeling environment is the quiet source of nearly every
slow afternoon. A library mismatch. A path that's right on one machine and wrong
on the next. An engine upgrade that takes a week to roll out and a month to
trust. Results that reproduce *almost* — which in a regulated world means they
don't reproduce at all.

Janus turns that environment into a versioned, portable container — a sealed box
that carries the engine, its dependencies, and its exact configuration with it.

- **Run a model** by selecting it and pressing go.
- **Change engine versions** by selecting a different container. No reinstall,
  no PATH surgery, no "works on my machine."
- **Get the same result everywhere** — the container is the single source of
  truth for the environment, so a run on a laptop and a run on the cluster are
  the same run.

For the modeler, this is the difference between *wrangling tools* and *doing
science*. For IT and QA, it's the difference between *hoping* an environment is
consistent and *knowing* it is, down to the image fingerprint recorded in the
log.

The container itself runs sealed: input files are handed in, results are handed
back, and sensitive material like license files is supplied at run time and
never left behind on disk. Reproducible **and** clean.

---

## It reads the model and knows what it is

You shouldn't have to tell a tool what kind of model you just handed it. Janus
figures that out for itself.

Point it at a model and it inspects the file — both the kind of file it is and
what's written inside — and recognizes the engine behind it. **NONMEM is the
primary target**, and it works the way modelers already expect: control streams
in, `.lst` and the usual output files out, run in place. The workflow you
already know, nothing relearned.

That recognition is by design not tied to one engine. The container layer
underneath can run essentially *any binary* — it was built to be engine-agnostic
from the start. The intelligence about *which* engine a model needs and *how*
it's best run lives in Janus's orchestration, sitting on top of that
general-purpose foundation. That separation is what leaves room for more engines
— Stan and Torsten among them — to follow without re-architecting anything.

The point is that the *engine is something Janus infers*, not something you
configure correctly every time. You bring the model; it works out the rest.

---

## Two faces, one tool

### For the PKPD modeler

You came to build models, not to babysit infrastructure.

- **Load and run.** Open a model, choose where it runs, press go. Output streams
  back to you live as it executes.
- **It already knows your engine.** Janus recognizes the engine from the model
  file itself — no telling it what you're running. NONMEM is the primary target
  and behaves exactly the way you're used to.
- **Run anywhere without changing how you work.** The same model goes to your
  local machine, an HPC grid, or an isolated container with no change to your
  workflow — just a different selection.
- **Never lose a run again.** Every execution is captured automatically — the
  command, the result, the outputs, the environment it ran in — and stored right
  alongside the model. Your run history travels with your work.
- **Iterate without merge pain.** Run history is stored so that two people
  working in parallel don't collide. No stepped-on logs, no conflict resolution
  over a results file.
- **Cross-platform, natively.** Windows, macOS, and Linux — the same application,
  not a port and an afterthought.

### For IT and QA

You're accountable for what the modelers can't see: consistency, traceability,
and the ability to *prove* it to an auditor.

- **Qualify the installation yourselves.** Janus ships with built-in
  Installation and Operational Qualification (IQ/OQ). It checks that it's
  installed and configured correctly, then runs a known reference model and
  confirms the answer matches an expected value within tolerance — and writes a
  timestamped, retained report you can put in the binder. No vendor visit, no
  consultant, no waiting.
- **Every run is signed and tamper-evident.** Execution records are
  cryptographically signed by the person who ran them, with their identity and a
  verifiable fingerprint attached. You can prove a record is genuine and
  unaltered after the fact.
- **The environment is auditable down to the image.** Because runs happen in
  versioned containers, the exact engine and configuration used for any result
  is recorded — not described, recorded.
- **Built for regulated reality, honest about its lane.** Janus produces the
  *execution journal* — what ran, how, and by whom — while your records of record
  stay in your validated systems and under your control. It's a tool that
  strengthens your quality process, not one that asks you to hand it over.
- **Deploy on your terms.** It works offline. There's no monthly phone-home to
  validate a license, which means airgapped and tightly controlled networks are
  first-class, not an exception you have to fight for.

---

## What makes it different

**Complexity collapses to a selection.** The single most technical, most
error-prone part of modeling — the runtime environment — becomes a dropdown.
That's the whole thesis, and everything else follows from it.

**One workflow, three places to run.** Local, grid, or container — the same
model, the same steps, your choice at the moment you press go. The tool adapts
to where your compute lives instead of forcing your work to move.

**It infers the engine for you.** Janus reads the model and recognizes the
engine behind it, so the right engine isn't a setting you maintain — it's
something the tool works out. NONMEM is the primary target and runs the way
modelers already expect, and because the execution layer underneath can run any
binary, support for additional engines extends naturally from the same
foundation.

**Reproducibility you can hand to an auditor.** Signed run logs, retained
qualification reports, and container-pinned environments mean "it reproduces" is
a thing you can demonstrate, not just assert.

**Self-service qualification.** Teams qualify their own installations on their
own hardware, on their own schedule. The compliance story is built in, not
bolted on or billed by the hour.

**Offline-first and yours.** No mandatory connection, no metering in the
background. It runs where you need it to run.

**Genuinely cross-platform.** One native application across Windows, macOS, and
Linux — so the modeler on a laptop and the validation server in the data center
are running the same thing.

---

## Who it's for

- **Pharmacometrics and PKPD groups** who want to spend their time on models
  instead of on the machinery around them — NONMEM today, with more engines on
  the way.
- **IT teams** supporting those groups who are tired of environment drift,
  per-machine installs, and "it worked yesterday."
- **QA and validation professionals** who need traceability, qualification, and
  tamper-evidence that they can produce on demand — and who'd rather own that
  process than rent it.

---

## Pricing

Simple, per-seat, per-year. Each tier includes everything in the one before it,
so you only step up when you need what the next tier adds.

| Tier           | Per seat / year | What it unlocks                                            |
| -------------- | --------------- | ---------------------------------------------------------- |
| **Basic**      | $200            | Local NONMEM execution on your own machine                 |
| **Advanced**   | $500            | Adds grid scheduling (SLURM, SGE) and the signed run log   |
| **Enterprise** | $1,250          | Adds container orchestration for reproducible environments |

**Basic** is for the modeler who just wants to run NONMEM locally and get
results, without the run-log and audit machinery.

**Advanced** is the step up for teams that run on a grid and need traceability —
this is where the signed, tamper-evident run log comes in, so every execution
leaves a verifiable record.

**Enterprise** adds the part that makes the environment itself a choice: container
orchestration, where switching engines or versions is as simple as selecting a
different container, and every run is reproducible down to the image.

---

## The throughline

Janus stands at the threshold between the science and the system that has to
trust it. Modelers get a tool that gets out of the way. IT and QA get a tool that
keeps a faithful, verifiable record. The container is the bridge between them:
one choice that makes a job easy to run *and* easy to prove.

That's the promise. Pick a container. Run your model. Keep the receipts.