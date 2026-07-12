# Janus Technical Overview

A technical appendix for investors and technical due diligence reviewers.

---

## Architecture at a Glance

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Janus Desktop GUI                           │
│                        (Go + Fyne.io)                               │
├─────────────────────────────────────────────────────────────────────┤
│                      Orchestrator Abstraction                       │
│         ┌──────────┬──────────┬──────────┬──────────┐              │
│         │  NONMEM  │   PSN    │   BBI    │  Hermes  │              │
│         │ (Direct) │          │          │ (gRPC)   │              │
│         └──────────┴──────────┴──────────┴──────────┘              │
├─────────────────────────────────────────────────────────────────────┤
│                      Execution Backends                             │
│    ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐   │
│    │  Local   │    │  SLURM   │    │   SGE    │    │  Docker  │   │
│    │          │    │ REST/CLI │    │          │    │ Container│   │
│    └──────────┘    └──────────┘    └──────────┘    └──────────┘   │
├─────────────────────────────────────────────────────────────────────┤
│                        Run Log Engine                               │
│              (Embedded artifacts, compression, signing)             │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Core Technology Decisions

### Why Go?

| Consideration | Go Advantage |
|---------------|--------------|
| **Single binary deployment** | No runtime dependencies, no Python/Java versioning issues |
| **Cross-compilation** | Build Windows/Linux/macOS from any platform |
| **Concurrency** | Goroutines for parallel job management, grid polling |
| **CGO support** | Fyne.io integration for native GUI rendering |
| **Static linking** | Predictable deployments in regulated environments |

### Why Fyne.io?

- Pure Go GUI framework (no Electron overhead)
- Native look-and-feel across platforms
- Single codebase for Windows, Linux, macOS
- Active development with 2.x stability

Trade-off: Fyne is less mature than Electron/Qt, but the single-binary deployment story is critical for pharmaceutical IT environments where installing dependencies is bureaucratically expensive.

---

## Hermes: The Container Orchestration Layer

Hermes is the key technical differentiator. It's a gRPC service that:

1. **Receives execution requests** — Model files, data, license as byte streams
2. **Manages container lifecycle** — Start, monitor, terminate
3. **Streams output** — Real-time stdout/stderr to GUI
4. **Collects artifacts** — Read specified file patterns, return as bytes
5. **Cleans up** — Ephemeral containers, no filesystem residue

### gRPC Contract (Simplified)

```protobuf
service HermesExecutor {
  rpc Execute(ExecuteRequest) returns (stream ExecuteResponse);
}

message ExecuteRequest {
  string image = 1;                    // Container image
  bytes license_data = 2;              // NONMEM license (runtime injection)
  repeated FileData files = 3;         // Model, data, includes
  string command = 4;                  // e.g., "nmfe76 model.mod model.lst"
  ResourceLimits resources = 5;        // CPU, memory, timeout
}

message ExecuteResponse {
  oneof payload {
    bytes stdout = 1;
    bytes stderr = 2;
    int32 exit_code = 3;
    FileData artifact = 4;             // Collected output file
  }
}
```

### Why This Matters for Validation

Traditional validation workflow:
```
Validate OS → Validate NONMEM install → Validate PSN → Validate grid config → ...
```

Hermes validation workflow:
```
Validate container image → Done
```

The container encapsulates the entire execution environment. IQ/OQ becomes:
- Image digest verification (SHA256)
- Known-result test execution
- Output comparison

No host-specific configuration to document. No environment variable audits. No path verification.

---

## Run Log Architecture

### Design Principles

1. **Immutability** — Entries append-only; no modification after creation
2. **Self-contained** — All artifacts embedded, not referenced
3. **Portable** — JSON format, backend-agnostic
4. **Signed** — Optional RSA signatures for CFR 21 Part 11

### Entry Structure

```json
{
  "id": "uuid-v4",
  "timestamp": "2024-01-15T14:30:00Z",
  "command": "nmfe76 model.mod model.lst",
  "exit_code": 0,
  "duration_ms": 45230,
  "user": "analyst@pharma.com",

  "files": {
    "model.mod": "H4sIAAAA...(gzip+base64)...",
    "model.lst": "H4sIAAAA...(gzip+base64)...",
    "model.ext": "H4sIAAAA...(gzip+base64)..."
  },

  "stdout": "H4sIAAAA...(gzip+base64)...",
  "stderr": "",

  "signature": "RSA-SHA256 signature (optional)",
  "certificate_chain": "..."
}
```

### Compression Performance

| Content Type | Typical Reduction |
|--------------|-------------------|
| NONMEM stdout (repetitive iteration output) | 97-98% |
| .lst files | 85-90% |
| Complete run record | 56-60% |

A 68KB raw record compresses to ~29KB. At scale (thousands of runs), this translates to significant storage savings.

---

## Grid Integration

### SLURM Support

Two modes:
1. **REST API** — Direct integration with slurmrestd (preferred)
2. **CLI fallback** — sbatch/squeue/scancel subprocess execution

REST configuration:
```yaml
slurm:
  mode: "REST"
  rest:
    socket_path: "/var/spool/slurm/restd/rest"
    api_version: "v0.0.36"
```

### SGE/TORQUE Support

CLI-based integration using qsub/qstat/qdel with output parsing.

### Job Lifecycle

```
Submit → Pending → Running → Completed/Failed
           ↓
      (Grid polling)
           ↓
    Update UI + Run Log
```

---

## Validation Strategy (IQ/OQ)

### Scope Boundaries

**What Janus validates:**
- Configuration → correct command generation
- UI input → expected system calls
- Grid output → accurate status parsing
- Run log → integrity, signatures, artifact embedding

**What Janus does NOT validate:**
- NONMEM mathematical correctness
- Grid scheduler configuration
- Network infrastructure

### Test Approach

```go
// Example: Validate command generation
func TestNONMEMCommandGeneration(t *testing.T) {
    executor := execution.NewNONMEMExecutor(config)
    binary, args, _ := executor.BuildCommand("model.mod", true, 4)

    assert.Equal(t, "/opt/NONMEM/nm76/run/nmfe76", binary)
    assert.Equal(t, []string{"model.mod", "model.lst"}, args)
}
```

This approach:
- Tests the **system call boundary** (what Janus sends to external tools)
- Uses **mock responses** for grid output parsing
- Maintains **clear responsibility boundaries**

---

## Security Considerations

### License Handling

- JWT-based licensing with embedded public key verification
- Offline validation (no phone-home requirement)
- NONMEM licenses injected at runtime, never persisted in containers

### Run Log Signing

- RSA-2048 signatures on run log entries
- User-generated key pairs (private key stays with user)
- Public key embedded in license for chain-of-trust verification

### Container Security

- No volume mounts (files sent as byte streams)
- Ephemeral containers (destroyed after execution)
- Optional image digest pinning for reproducibility

---

## Development Practices

### No Global State

```go
// Factory function pattern - no init()
func Command() *cobra.Command {
    var cfg *config.Config

    return &cobra.Command{
        PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{}),
        RunE: func(c *cobra.Command, args []string) error {
            return execute(cfg)  // Dependency injected
        },
    }
}
```

### CI/CD Pipeline

- GitHub Actions with container-based builds
- golangci-lint v2.1.5 enforcement
- Unit, integration, and GUI test suites
- Cross-platform artifact generation (Windows MSI, Linux packages, macOS bundles)

---

## Roadmap (Technical)

| Milestone | Components |
|-----------|------------|
| **Current** | NONMEM execution, SLURM integration, run log with signing |
| **Near-term** | Hermes container orchestration, PostgreSQL run log backend |
| **Medium-term** | Stan/Monolix executors, Kubernetes scheduler backend |
| **Long-term** | MCP server for AI integration, distributed run log (S3) |

---

## Code Quality Indicators

- **Test coverage**: Unit + integration + validation suites
- **Static analysis**: golangci-lint with custom ruleset
- **Documentation**: Architecture docs, API contracts, validation guides
- **No global variables**: Strict dependency injection

---

## Questions for Technical Due Diligence

1. **Container runtime requirements** — Docker, Podman, or both?
2. **Grid scheduler priorities** — SLURM-first, or equal SGE/TORQUE investment?
3. **Run log backend scaling** — What's the expected volume (runs/day, artifact sizes)?
4. **Multi-modal timeline** — Customer demand for Stan/Monolix support?

---

*For additional technical detail, see the main [README.md](../README.md) and [CLAUDE.md](../CLAUDE.md).*