# Summary

Execution in Janus is currently modeled as a single flat list of "modes":

* NONMEM
* BBI
* PSN
* HERMES

This flat list conflates two **orthogonal** concepts and is the source of the
confusion this proposal exists to resolve. HERMES in particular feels out of
place sitting next to NONMEM/PSN/BBI — because it isn't the same *kind* of thing.

# The Two Axes

Execution is really the product of two independent axes:

## Axis 1 — Engine (the "what")

The command structure used to invoke a NONMEM run:

* **NONMEM** — invoked directly
* **PSN** — PsN wrapper around NONMEM
* **BBI** — `bbi` command structure around NONMEM

These are peers. BBI is **not** a destination — it is simply another command
structure for executing NONMEM, and is treated exactly like NONMEM and PSN.

## Axis 2 — Destination (the "where")

Where that command actually runs:

* **Here** — the local system; reads the engine's local config
* **Scheduler** — submits to the configured HPC scheduler (SLURM/SGE/TORQUE)
* **Hermes** — delegates execution to a Hermes instance, which itself fans out
  into an orchestrator:
  * **Local Docker** (current behavior)
  * **Kubernetes** (new — see below)

# The Refactor

Cross the two axes. **Every engine can target every destination.** Hermes stops
being a sibling of the engines and becomes a destination with its own
orchestrator sub-choice.

| Engine ↓ \ Destination → | Here | Scheduler | Hermes: Docker | Hermes: K8s |
|--------------------------|:----:|:---------:|:--------------:|:-----------:|
| NONMEM                   |  ✓   |     ✓     |       ✓        |   ✓ (new)   |
| PSN                      |  ✓   |     ✓     |       ✓        |   ✓ (new)   |
| BBI                      |  ✓   |     ✓     |       ✓        |   ✓ (new)   |

Selection reads as: `<Engine> -> <Destination> [ -> <Orchestrator> ]`, e.g.

1. `NONMEM -> Here`
2. `PSN -> Scheduler`
3. `BBI -> Hermes -> Kubernetes`

# Kubernetes Destination

This orchestrator does not exist yet; there is no reason not to build it. Its job
is to orchestrate a pod in the **same way** Hermes-over-Docker does, except the
pod lives in a remote namespace and we reach it via a port-forward.

## New config items

* **Kube config** — path to kubeconfig (defaults to the standard `~/.kube/config`) — *may provide*
* **Context** — kube context to use — *may provide*
* **Namespace** — target namespace — *must provide*
* **Image** — Hermes image to run — *must provide*

## Flow

1. Provision the Hermes pod into the target namespace with the requested image
2. Wait for it to become healthy/ready
3. Port-forward the pod's exposed port to localhost via native k8s tooling
4. Connect to Hermes over the forward
5. Request execution
6. Stream results into the run log
7. Collect artifacts into the run log
8. Tear down the pod

# Deliverables

This collapses into two pieces of work:

1. **UI redesign** — present execution as **Engine × Destination**, organizing
   around the destination as the primary "where" concept rather than a flat mode
   list. (Tracked separately under milestone 3.)
2. **Add Kubernetes execution** — the new Hermes orchestrator described above.
   (Tracked by issue #13.)
