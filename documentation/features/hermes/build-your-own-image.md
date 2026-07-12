# Build Your Own NONMEM Image for Hermes

## Why you need to build your own

NONMEM is licensed software. Pharmalytica **cannot distribute container images that
contain NONMEM or a NONMEM license**. What we *can* give you is a common, supported
recipe: extend the Hermes base image with your own licensed NONMEM installation, push
the result to a registry **you** control, and point Janus at it.

If you don't care whether the image is based on Debian or anything else, use the
[`Dockerfile.nonmem.template`](./Dockerfile.nonmem.template) in this directory as your
starting point — it is the same pattern Janus uses for its internal test image, minus
the mock.

## What "Hermes-compatible" means

Janus does not run NONMEM directly. It talks to a small gRPC **execution server**
("Hermes") inside the container on port `50051`. The simplest way to get that server is
to base your image on the Hermes base image:

```dockerfile
FROM ghcr.io/pharmalytica/hermes:main-trixie
```

Everything else — your NONMEM tree, the run-script path, the discovery labels — layers
on top. See the template for the full file.

## Steps

1. **Place your licensed NONMEM installation** in `./nonmem` next to the Dockerfile
   (or adjust the `COPY` line to match your layout).
2. **Build** it, tagging for your own registry:
   ```bash
   docker build -f Dockerfile.nonmem.template -t my.registry/nonmem:7.5.1 .
   ```
3. **Push** to your registry:
   ```bash
   docker push my.registry/nonmem:7.5.1
   ```
   If the registry is private, see [`DOCKER_PRIVATE_REPOS.md`](../../DOCKER_PRIVATE_REPOS.md)
   for authentication.
4. **Point Janus at it** (next section).

## Auto-discovery labels

Janus scans local Docker images and lists compatible ones in the Hermes config image
selector. For your image to be discovered it **must** carry these two labels:

| Label | Required value | Purpose |
|-------|----------------|---------|
| `io.pharmalytica.janus.type` | `executor` | Marks the image as a Janus executor |
| `io.pharmalytica.janus.platform` | `nonmem` | Identifies the platform |

These labels improve the experience but are optional:

| Label | Purpose |
|-------|---------|
| `io.pharmalytica.janus.display-name` | Friendly name shown in the selector |
| `io.pharmalytica.janus.description` | Description shown in the selector |
| `io.pharmalytica.janus.container_command` | Auto-fills the command path in the dialog |
| `io.pharmalytica.janus.version` / `…min-janus-version` | Version metadata |
| `io.pharmalytica.nonmem.version` / `…compiler` / `…compiler-version` | NONMEM build metadata |

(The canonical list lives in `internal/container/types.go`.) You can always type any
image reference into the selector by hand instead of relying on discovery.

## Wiring it into `.janus.config.json`

Each model directory carries a `.janus.config.json`. The `hermes` block selects your
image and the command to run inside it:

```json
{
  "hermes": {
    "image": "my.registry/nonmem:7.5.1",
    "container_command_path": "/opt/NONMEM/nm75/run/nmfe75",
    "resources": {
      "cpu_cores": 4,
      "memory": "8Gi"
    }
  }
}
```

- `image` — the tag you pushed in step 3.
- `container_command_path` — the in-container path to the NONMEM run script (the same
  value as the `io.pharmalytica.janus.container_command` label). Adjust the
  `nm75/nmfe75` portion to your NONMEM version.
- `resources` — CPU/memory for the container (`cpu_cores` is a positive integer;
  `memory` uses forms like `8Gi`, `2048Mi`, `1G`). The schema is defined in
  `internal/config/hermes_model.go`.

---

## Appendix: portal "Hermes" tab — content contract

> The customer-facing **portal is a separate web application (different repo)**. The
> portal's informational "Hermes" tab is implemented there; this section is the content
> contract so it can render/link the canonical instructions above.

The portal's Hermes tab should present (read-only, informational):

1. **Why self-build** — the licensing explanation from the top of this document.
2. **The Dockerfile template** — rendered from / linked to
   `documentation/features/hermes/Dockerfile.nonmem.template` (single source of truth;
   do not fork the content into the portal).
3. **Build & push steps** — the numbered steps above, including the private-registry
   pointer.
4. **Required vs optional labels** — the two discovery tables above.
5. **`.janus.config.json` mapping** — the JSON example and field notes above.

Suggested implementation in the portal repo: fetch these files at build time (or vendor
them) rather than duplicating the text, so the instructions stay in lockstep with the
labels/config the Janus client actually enforces. The actual tab UI is tracked in the
portal repo and is out of scope for the Janus repo.
