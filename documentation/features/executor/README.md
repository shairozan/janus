# Executor CLI

The `executor` binary is a lightweight, standalone container orchestration proxy that provides transparent drop-in replacement functionality for pharmacometric modeling tools.

## Overview

The executor automatically detects the modeling platform from the input file and executes it within a containerized environment via Hermes transport. It requires no tool-specific configuration—just point it at a model file and it handles the rest.

### Key Characteristics

- **Zero-dependency design**: Uses only Go standard library (no Cobra/Viper)
- **Statically linked**: Portable across Linux, macOS, and Windows
- **Transparent pass-through**: Does not interpret tool-specific arguments
- **Drop-in replacement**: Acts as a shim around modeling tools
- **Fail-fast architecture**: Validates configuration and license before Docker operations

## Supported Platforms

| Platform | Extensions | Detection Method |
|----------|------------|------------------|
| **NONMEM** | `.mod`, `.ctl`, `.nmctl` | `$PROBLEM` + `$DATA` directives |
| **Stan** | `.stan` | `data {}`, `parameters {}`, `model {}` blocks |
| **Torsten** | `.stan` | Stan + Torsten functions (`PKModelOneCpt`, `pmx_solve_*`) |
| **Monolix** | `.mlxtran` | `<DATAFILE>`, `<MODEL>`, `<FIT>` tags |
| **Unknown** | Any | Pass-through mode with minimal assumptions |

## How Inference Works

The executor uses a two-phase detection algorithm:

1. **Extension-based detection** (fast path): Checks file extension to narrow down candidates
2. **Content-based detection** (fallback): Scans file contents for platform-specific markers

If detection fails, the executor proceeds in pass-through mode rather than failing.

## Usage

```bash
# Basic usage - auto-detects platform
executor model.mod model.lst

# Stan with tool-specific options
executor model.stan --algorithm=hmc --num_samples=2000

# Explicit config location
executor --executor-hermes-config /shared/config.json model.mod

# Quiet mode (buffer output)
executor --executor-quiet model.mod model.lst
```

## Executor-Specific Flags

Flags prefixed with `--executor-` are consumed by the executor and never passed to the container:

| Flag | Description |
|------|-------------|
| `--executor-help` | Show help |
| `--executor-version` | Show version |
| `--executor-janus-config PATH` | Janus config for Docker socket settings |
| `--executor-hermes-config PATH` | Explicit Hermes config location |
| `--executor-quiet` | Suppress real-time output streaming |
| `--executor-no-runlog` | Skip run log update |

All other arguments are passed directly to the container tool.

## Configuration Discovery

The executor searches for `.janus.config.json` using a 4-step heuristic:

1. Explicit `--executor-hermes-config` flag (highest priority)
2. Adjacent to any file path in the arguments
3. Current working directory
4. Error with helpful hints

### Required Configuration

**`.janus.config.json`** (must exist adjacent to model):

```json
{
  "image": "shairozan/hermes-nonmem:nm76",
  "container_command_path": "/opt/NONMEM/nm76/run/nmfe76",
  "resources": {
    "cpus": 4,
    "memory": "8Gi"
  }
}
```

## Execution Flow

```
Parse executor flags
    ↓
Discover .janus.config.json
    ↓
Verify license (fail fast)
    ↓
Detect model platform
    ↓
Build workspace (model + dependencies)
    ↓
Execute via Hermes gRPC transport
    ↓
Stream output / Update run log
    ↓
Return exit code
```

## Hermes Integration

The executor integrates with Janus's Hermes container execution infrastructure:

- Builds workspace payload from model and all dependencies
- Creates containerized execution environment
- Streams stdout/stderr in real-time
- Handles signals (SIGINT/SIGTERM) gracefully
- Supports custom Docker socket configuration

## License Verification

The executor validates the Janus license before any Docker operations:

- Verifies JWT signature using embedded public key
- Checks expiration date
- Supports multiple locations: env var, home directory, current directory

## Run Log Integration

Optionally records execution metadata to `.janus.runlog.json`:

- Exit code, duration, resource usage
- Captured stdout/stderr
- Non-blocking: failures don't stop execution

Use `--executor-no-runlog` to disable.

## See Also

- [Container Detection](../container_detection/) - Auto-discovery of Hermes-compatible images
- [Hermes Categorization](../hermes_categorization/) - Model platform detection details
