# Janus: NONMEM Grid Management Tool

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

An open-source alternative to Certara Pirana, built with Go + Fyne.io, with
pluggable orchestrator backends and a cryptographically signed execution record
suited to CFR 21 Part 11 environments.

Every run is signed (RSA-SHA256), attributed to the identity bound to your signing
key, and — for containerized executions — pinned to an image digest, so the
evidence a sponsor needs is produced as a by-product of running the model rather
than reconstructed afterward. Janus is a GAMP Category 4 tool that supports your
Part 11 obligations; it is not itself a Part 11 system of record. See
[documentation/gxp/INTENDED_USE.md](documentation/gxp/INTENDED_USE.md) for the
exact boundary, and note that as community-supported open source there is no
vendor validation package behind it — the qualification burden is yours, and
Janus gives you IQ/OQ tooling to discharge it.

**No license key, no activation, no phoning home.** Clone it, build it, run it.

## Project Overview

**Goal**: A GUI tool that doesn't require license validation or punch holes in your firewall just to submit modeling jobs.

**Architecture**: Clean separation between GUI, orchestrator abstraction layer, and backend implementations (starting with PSN, expanding to BBI).

## Core User Journeys

1. **Submit Jobs**: Modelers can submit NONMEM jobs to SLURM/SGE grids with proper validation
2. **View Jobs**: See only your own job details (filtered by user ownership)
3. **Cancel Jobs**: Cancel only jobs you own (built-in authorization)
4. **Execution History**: Browse personal job history for record-keeping and command reuse
5. **Generate IQ/OQ**: Sysadmins can auto-generate validation documentation

## Development Setup (Windows)

### Prerequisites

- **Go 1.25+**
- **A C compiler** — Fyne needs cgo
- **git** *(optional)* — only used to suggest an identity when you run
  `janus keys generate`. Janus never shells out to git while running, so compute
  nodes and containers do not need it.

### The Compiler Saga

**TL;DR**: Use clang, not GCC 15.2.0.

If you just want to get started:

```bash
# Install MSYS2 (if you don't have it)
# Download from https://www.msys2.org/

# In MSYS2 terminal, install clang
pacman -S mingw-w64-x86_64-clang

# Set environment variables (add to your shell profile)
export CC=clang
export CXX=clang++
export CGO_ENABLED=1
```

### The Long Story

We discovered that Go 1.25 + CGO + GCC 15.2.0 (latest MSYS2) = `cgo.exe: exit status 2`. This is a known compatibility issue with bleeding-edge GCC versions.

**What we tried:**
- Build tags (`-tags windows`)
- Environment variables (`CGO_ENABLED=1`, `GOOS=windows`)
- Older GCC versions
- Sacrificing a rubber duck to the CGO gods

**What worked:**
- Using clang instead of GCC
- Setting `CC=clang` and `CXX=clang++`
- Accepting that sometimes the newest isn't the bestest

### Installation Steps

1. **Install tools:**
```bash
# Fyne setup checker
go install fyne.io/setup@latest

# Fyne CLI tools
go install fyne.io/fyne/v2/cmd/fyne@latest
```

2. **Verify setup:**
```bash
$(go env GOPATH)/bin/setup
```

3. **Clone and run:**
```bash
git clone <repo>
cd janus
export CC=clang
export CXX=clang++
go run main.go gui
```

### GoLand Configuration

The run configurations are pre-configured with the correct environment variables:
- `CC=clang`
- `CXX=clang++` 
- `CGO_ENABLED=1`

Just use the "Janus GUI" run configuration.

## Architecture Principles

1. **No global variables** - Everything is dependency injected
2. **Orthogonal architecture** - Services configured at startup, passed down
3. **Cobra + Viper** - Standard CLI framework, even for GUI apps
4. **Single execution channel** - All job submissions flow through one message bus for run logging/hooks

## Usage

### Running Janus

**Launch the GUI** (no arguments):
```bash
janus
```

**Open a specific model file**:
```bash
# Positional argument (recommended for file associations)
janus model.mod

# Or using flag (backward compatibility)
janus --model model.mod
```

**Supported model file extensions**:
- `.mod` - NONMEM model files
- `.ctl` - NONMEM control stream
- `.nmctl` - NONMEM control stream

**Note**: Once native installers are available (Windows MSI, Linux DEB, macOS signed binaries), you'll be able to double-click model files to open them directly in Janus. See [installers_execution_plan.md](documentation/design/features/implemented/installers/installers_execution_plan.md) for details.

### Other Commands

```bash
# Show version
janus version

# Initialize Hermes configuration
janus hermes init

# Execute a model
janus execute model.mod
```

# Developer Experience

This was originally developed purely on linux, then moved to windows, which exposed how clunky windows can be. The mage architecture has several commands added to help this _still work_. `mage docker:lint` will run lint in a container (the same that CI is using) to check for linting rules.

## Docker Development

Building in Docker pins the toolchain for you, which is handy on Windows. No
credentials are needed — every dependency is public.

```bash
mage docker:buildDev     # build the dev image (first time only, 5-10 minutes)
mage docker:build        # build with cached deps
mage docker:unit         # unit tests
mage docker:gui          # GUI tests
mage docker:checkAll     # full validation
```

## Project Structure

```
janus/
├── cmd/                    # Cobra commands
│   ├── gui/               # GUI command
│   └── root.go            # Root command setup
├── internal/
│   └── gui/               # Fyne GUI implementation
├── main.go                # Entry point
└── go.mod                 # Dependencies
```

## Configuration Examples

### Example Metworx SLURM Details

For Metworx environments, use these SLURM REST API configuration settings:

```yaml
# Metworx SLURM Configuration
scheduler: "SLURM"

slurm:
  mode: "REST"
  timeout: "30s"
  rest:
    socket_path: "/var/spool/slurm/restd/rest"
    api_version: "v0.0.36"
    timeout: "30s"
```

### Alternative SLURM Configurations

**Standard SLURM REST (newer versions):**
```yaml
slurm:
  mode: "REST"
  rest:
    socket_path: "/var/run/slurm/slurmrestd.sock"
    api_version: "v0.0.40"
    timeout: "30s"
```

**SLURM CLI Mode (fallback):**
```yaml
slurm:
  mode: "CLI"
  timeout: "30s"
```

### Complete Configuration Example

```yaml
# Janus Configuration File
organization: "Metworx User"
default-directory: "~/models"
nonmem-path: "/opt/NONMEM/nm76/run"
nonmem-binary: "nmfe76"

# Scheduler Configuration
scheduler: "SLURM"

slurm:
  mode: "REST"
  timeout: "30s"
  rest:
    socket_path: "/var/spool/slurm/restd/rest"
    api_version: "v0.0.36"
    timeout: "30s"

# Execution Configuration
execution-mode: "NONMEM"

# Feature Flags
runlog-enabled: true
projects: true

# Validation Configuration for CFR 21 Part 11 compliance
validation:
  iq: "~/.config/janus/validation/iq-report.json"
  oq: "~/.config/janus/validation/oq-report.json"
```

## Parameter Correlation Strategies

When comparing runs, Janus needs to match parameters across models. This is straightforward when parameter counts match, but becomes complex when:
- Parameters are added/removed between runs
- Labels are inconsistent or missing
- Parameter ordering changes

Janus provides three correlation strategies to handle these scenarios:

### Strategy Overview

| Strategy | Matching Method | Best For | Trade-off |
|----------|----------------|----------|-----------|
| **Conservative** (default) | Label-only | Labeled models, regulatory work | May fragment rows for unlabeled params |
| **Inferred** | Labels + position fallback | Mixed labeling, iterative development | Assumes positional equivalence for unlabeled |
| **Positional** | Position-only | No labels, quick comparisons | Ignores label information entirely |

### Conservative Strategy (Default)

Only correlates parameters with matching labels. Parameters without labels are treated as unrelated, even if they appear at the same position.

**Example:**
```
Run 1: THETA1 (CL), THETA2 (V), THETA3 (no label)
Run 2: THETA1 (CL), THETA2 (V), THETA3 (no label)
```

Result:
- THETA1/THETA2: **Correlated** (matching labels "CL", "V")
- THETA3: **Unrelated** in each run (no label to match)

**Use when:**
- Labels are consistently applied across models
- Regulatory submissions requiring traceable parameter matching
- You prefer false negatives over false positives

### Inferred Strategy

Uses labels when available, falls back to positional matching for unlabeled parameters.

**Example:**
```
Run 1: THETA1 (CL), THETA2 (no label), THETA3 (no label)
Run 2: THETA1 (CL), THETA2 (no label), THETA3 (no label)
```

Result:
- THETA1: **Known correlation** (matching label "CL")
- THETA2/THETA3: **Inferred correlation** (same position, no labels)

**Use when:**
- Iterative model development with partial labeling
- Comparing runs where labels were added later
- You trust positional equivalence for unlabeled params

### Positional Strategy

Ignores labels entirely and correlates purely by parameter position.

**Example:**
```
Run 1: THETA1 (CL), THETA2 (V)
Run 2: THETA1 (Ka), THETA2 (Vmax)  # Different labels!
```

Result:
- THETA1: **Inferred correlation** (same position, labels ignored)
- THETA2: **Inferred correlation** (same position, labels ignored)

**Use when:**
- Labels are known to be unreliable or outdated
- Quick comparisons where label accuracy isn't critical
- Migrating from legacy models without labels

### Configuration

The correlation strategy is configured per-model in `.janus.config.json`:

```json
{
  "image": "shairozan/hermes-nonmem:nm76",
  "container_command_path": "/opt/NONMEM/nm76/run/nmfe76",
  "resources": {
    "cpu_cores": 4,
    "memory": "8Gi"
  },
  "correlation_strategy": "inferred"
}
```

Valid values for `correlation_strategy`:
- `"conservative"` (default) - Label-only matching
- `"inferred"` - Labels + positional fallback
- `"positional"` - Position-only matching

If omitted or invalid, defaults to `conservative`.

### API Usage

```go
import "github.com/shairozan/janus/internal/comparison"

// Default (conservative)
result, err := comparison.CompareRuns(records)

// With explicit strategy
result, err := comparison.CompareRunsWithStrategy(records, comparison.StrategyInferred)

// Parse from string (for config/UI)
strategy := comparison.ParseCorrelationStrategy("inferred")
result, err := comparison.CompareRunsWithStrategy(records, strategy)
```

### Visual Indicators

The comparison UI displays badges to indicate correlation status:
- `[KNOWN]` - Matched by label (high confidence)
- `[INFER]` - Matched by position (medium confidence)
- `[NEW]` - Parameter only in one run (no correlation)

### Label Best Practices

For reliable parameter correlation:

1. **Add labels to control streams:**
   ```
   $THETA
   (0, 2)   ; CL
   (0, 10)  ; V
   (0, 0.5) ; Ka
   ```

2. **Use consistent naming across runs:**
   - Same label = same parameter
   - Case-insensitive matching (CL = cl = Cl)

3. **Double-comment style supported:**
   ```
   $OMEGA
   0.05  ;; IIV_CL
   ```

### Hermes Container Execution Configuration

For containerized execution using Hermes (gRPC-based container proxy):

```yaml
# Janus Configuration with Hermes Execution
organization: "Acme Pharmaceuticals"
default-directory: "~/models"

# Execution Configuration
execution-mode: "HERMES"

# Hermes Configuration
hermes:
  # Container image with NONMEM and dependencies
  image: "shairozan/hermes-nonmem:nm76"

  # NONMEM license file configuration
  # CRITICAL: This file is required for NONMEM execution
  license:
    path: "~/.config/janus/nonmem.lic"

  # Default resource limits
  resources:
    cpus: "4"
    memory: "8G"
    timeout: "24h"

  # Container lifecycle settings
  container:
    port: 50051
    cleanup: true
    startup_timeout: "30s"

  # Default file patterns to collect after execution. This is the INHERITED
  # DEFAULT: a model's per-model .janus.config.json `retain` (nested under the
  # `hermes` key) is authoritative and overrides this. When a model sets none,
  # this global default applies; when it too is empty, the engine's category
  # defaults apply. See `janus hermes init` / the GUI Hermes Config dialog to set
  # per-model retain.
  retain:
    - "*.lst"
    - "*.ext"
    - "*.tab"
    - "*.cov"
    - "*.cor"
    - "*.coi"
    - "*.phi"
```

**Key Features**:
- Self-contained execution with all files sent as byte streams
- NONMEM license injected at runtime (never persisted in container)
- Automatic container cleanup after execution
- Streamed STDOUT/STDERR to audit log
- Artifact collection via gRPC response

## Current Status

- ✅ Basic GUI structure (tabs, layout, placeholders)
- ✅ Windows development environment setup
- ✅ Run configurations for GoLand
- ✅ SLURM REST API integration with setup wizard
- ⏳ Service layer (execution channels, run log engine)
- ⏳ Project management
- ⏳ MCP server for AI integration

## Future Plans

- **Projects**: Define model/data/metadata, track run history
- **Run log engine**: Pluggable backends (filesystem, PostgreSQL, S3)
- **External hooks**: Environment variable export + script execution
- **MCP server**: Query run history via AI chat
- **Multi-platform**: Linux primary, Windows/Mac secondary
- **Supply Chain Security**: GitHub attestations (build provenance, SBOM) for enterprise trust and compliance

## Considerations for Cleanup

NONMEM executions generate numerous temporary and output files. Understanding which to keep is critical for storage management and documentation compliance.

### File Categories

**Always Safe to Delete (Build Artifacts):**
- `FCON`, `FMSG`, `FREPORT`, `FSTREAM` - NONMEM internal communication files
- `FSIZES`, `PRSIZES.f90` - Compilation size files
- `FSUBS`, `FSUBS2`, `FSUBS.f90` - Generated FORTRAN subroutines
- `INTER` - Interactive mode file
- `LINK.LNK`, `LINKC.LNK`, `compile.lnk` - Compiler linking files
- `gfortran.txt` - Compiler output logs
- `nonmem` - Compiled executable (temporary)
- `temp_dir/` - Temporary working directory
- `nmprd4p.mod` - FORTRAN module file
- `.*.settings.grid.json` - Janus temporary grid settings

**Potentially Valuable (Diagnostics):**
- `*.grd` - Gradient information (optimization diagnostics)
- `*.cpu` - CPU time tracking (performance analysis)
- `*.shk` - Shrinkage estimates (useful for model diagnostics)
- `*.shm` - Shrinkage matrix (less commonly needed)
- `FDATA.csv` - CSV-formatted data (convenience, regenerable)

**Must Keep (Core Results):**
- `*.mod` - Model source code (essential)
- `*.csv` - Input data file (essential)
- `*.lst` - List file with complete results (primary output)
- `*.ext` - Parameter estimates table (essential for analysis)
- `*.phi` - Individual parameter estimates (essential for post-processing)
- `*.xml` - Structured output (essential for programmatic parsing)
- `FDATA` - NONMEM-formatted data (may be needed for re-runs)
- `*.janus_history.json` - Execution run log (compliance documentation)

### Storage Strategy

**Traditional Approach (BBI/PSN):**
- Create subdirectories for each run
- Archive entire directory trees
- Results in significant storage overhead

**Janus Approach (Implemented):**
- Gzip + Base64 compression for all text fields (stdout, stderr, descriptions)
- Essential output files (`.lst`, `.ext`, `.phi`, `.xml`, `.mod`) embedded directly into run log
- Automatic compression on run completion
- **56-98% storage reduction** depending on content (typical: 60% overall)
- Single source of truth for compliance and analysis
- Enables centralized run log database (PostgreSQL, S3) without filesystem dependencies
- Backward compatible: reads both compressed and legacy uncompressed records

**Benefits:**
- Complete run log with embedded results
- No orphaned files after cleanup
- Easy migration between storage backends
- Simplified backup procedures
- Cloud-friendly (no need to preserve filesystem structure)

### Cleanup Workflow (Future)

```bash
# After successful execution
janus cleanup --run-id <id> --keep-essential

# Options:
#   --keep-essential: Keep only .mod, .csv, run log (embedded results)
#   --keep-diagnostics: Also keep .grd, .shk, .cpu files
#   --dry-run: Show what would be deleted
```

The run log becomes the single source of truth, containing:
- Execution metadata (command, exit code, status)
- Input files (`.mod` embedded, `.csv` reference)
- Output files (`.lst`, `.ext`, `.phi`, `.xml` embedded and compressed)
- Complete stdout/stderr (compressed, ~98% reduction on repetitive output)
- User descriptions/notes (compressed)
- Performance metrics (cores, timing)
- User/timestamp information
- Complete command reconstruction

**Compression Performance:**
- Repetitive NONMEM output: **97-98% compression** (42KB → 1KB typical)
- Complete run record (all files + stdout): **56-60% overall** (68KB → 29KB typical)
- Empty/small fields: not compressed (efficiency optimization)
- Backward compatible: seamlessly reads old uncompressed records

## Docker Execution Model (Proposed)

### Overview

A new execution model that provides containerized, isolated execution environments for pharmacometric modeling. This complements existing NONMEM, PSN, and BBI execution modes by enabling reproducible, portable execution with complete environment isolation.

**Key Design**: The entire execution environment is self-contained within the gRPC request. License data, model files, and datasets are sent as byte streams (similar to a zip archive), eliminating dependencies on host filesystem paths or volume mounts. Results are returned the same way - all output files as byte streams in the response.

### Architecture

**gRPC-Based Container Proxy**

The Docker execution model uses a gRPC service that acts as a bridge between Janus and containerized execution environments:

```
┌─────────────────┐    gRPC     ┌─────────────────┐    Docker API   ┌─────────────────┐
│  Janus Client   │◄────────────┤  gRPC Proxy     │◄───────────────►│ Docker Container│
│  (Orchestrator) │             │  Service        │                 │ (Isolated Env)  │
└─────────────────┘             └─────────────────┘                 └─────────────────┘
        │                               │                                    │
        │ 1. Submit Job Request         │                                    │
        │   (License + Model Files      │                                    │
        │    + Data as byte streams)    │                                    │
        ├──────────────────────────────►│                                    │
        │                               │ 2. Create Container                │
        │                               ├───────────────────────────────────►│
        │                               │                                    │
        │                               │ 3. Write Files Inside Container    │
        │                               │    (from byte streams)             │
        │                               ├───────────────────────────────────►│
        │                               │                                    │
        │                               │ 4. Execute Command                 │
        │                               ├───────────────────────────────────►│
        │                               │                                    │
        │                               │ 5. Stream STDOUT/STDERR            │
        │                               │◄───────────────────────────────────┤
        │ 6. Stream Logs to UI          │                                    │
        │◄──────────────────────────────┤                                    │
        │                               │                                    │
        │                               │ 7. Collect Exit Code + Read Files  │
        │                               │◄───────────────────────────────────┤
        │                               │                                    │
        │ 8. Return Results + Artifacts │                                    │
        │   (Files as byte streams)     │                                    │
        │◄──────────────────────────────┤                                    │
```

### gRPC Service Contract

**Input (Janus → Container)**:

1. **License File/Data**: Software licensing information (e.g., NONMEM license)
2. **Model Files**: Control stream, data files, and directory structure
3. **Execution Command**: Command to run inside container (e.g., `nmfe76 model.mod model.lst`)
4. **Environment Variables**: Container environment configuration
5. **Resource Limits**: CPU, memory, timeout constraints

**Output (Container → Janus)**:

1. **STDOUT Stream**: Real-time standard output from execution
2. **STDERR Stream**: Real-time standard error from execution
3. **Exit Code**: Process exit status
4. **File Artifacts**: Byte slices of all files in audit log (results, logs, diagnostics)
5. **Execution Metadata**: Runtime, resource usage, container info

### gRPC Service Definition (Conceptual)

```protobuf
service DockerExecutor {
  // Submit a job and stream execution updates
  rpc ExecuteJob(JobRequest) returns (stream JobUpdate);

  // Retrieve artifacts after execution
  rpc GetArtifacts(ArtifactRequest) returns (stream FileChunk);

  // Cancel a running job
  rpc CancelJob(CancelRequest) returns (CancelResponse);
}

message JobRequest {
  bytes license_data = 1;
  repeated FileData model_files = 2;
  string command = 3;
  map<string, string> environment = 4;
  ResourceLimits limits = 5;
  string container_image = 6;
}

message JobUpdate {
  oneof update {
    LogLine stdout_line = 1;
    LogLine stderr_line = 2;
    ExitCode exit_code = 3;
    ExecutionError error = 4;
  }
}

message FileData {
  string path = 1;
  bytes content = 2;
  int32 mode = 3;
}

message ArtifactRequest {
  string job_id = 1;
  repeated string file_patterns = 2;  // e.g., ["*.lst", "*.ext"]
}

message FileChunk {
  string path = 1;
  bytes chunk = 2;
  bool is_final = 3;
}
```

### Benefits

**Reproducibility**:
- Fixed, versioned execution environments
- Consistent results across machines and platforms
- Complete dependency isolation

**Security**:
- Sandboxed execution prevents system contamination
- License data never persists on disk (memory-only)
- Container ephemeral by default

**Portability**:
- Run identical environments on Windows, Linux, macOS
- Easy sharing of execution environments via Docker images
- No local software installation required (except Docker)
- Self-contained execution - no filesystem mounts or path dependencies
- Works across network boundaries (remote gRPC proxy)

**Audit & Compliance**:
- Complete capture of execution environment (image digest, versions)
- File-level artifact collection for validation
- Integration with existing Janus audit engine

**Resource Management**:
- CPU/memory limits enforced at container level
- Timeout-based cleanup
- Parallel job execution with isolated resources

### Use Cases

1. **Regulatory Validation**: Fixed execution environments for reproducible IQ/OQ results
2. **Cross-Platform Development**: Develop on Windows, execute in Linux containers
3. **Legacy Software**: Run older NONMEM versions without conflicting installations
4. **Cloud Execution**: Easy integration with Kubernetes/ECS for scalable compute
5. **Environment Isolation**: Multiple NONMEM versions side-by-side

### Implementation Considerations

**Container Images**:
- Pre-built images with NONMEM + dependencies
- User-provided custom images
- Image verification (digest pinning)

**License Handling**:
- License data injected at runtime (not baked into images)
- Support for node-locked and floating licenses
- License file cleanup after execution

**File Management**:
- All files sent as byte streams in gRPC request (no volume mounts required)
- Self-contained "zip archive" approach - license, model files, data files all bundled
- No dependency on host filesystem paths or locations
- Artifact collection via streaming byte chunks in response
- Container filesystem ephemeral - everything destroyed after execution

**Integration Points**:
- New `execution-mode: "DOCKER"` configuration option
- Compatible with existing scheduler backends (SLURM can launch Docker jobs)
- Grid integration: Submit Docker execution to SLURM/SGE nodes

### Configuration Example

```yaml
# Janus Configuration with Docker Execution
organization: "Acme Pharmaceuticals"
default-directory: "~/models"

# Execution Configuration
execution-mode: "DOCKER"

docker:
  # gRPC proxy service
  proxy:
    address: "localhost:50051"
    timeout: "30s"
    tls: true
    cert_path: "~/.config/janus/docker-proxy.crt"

  # Default container image
  default_image: "your-registry/nonmem:nm76"

  # Resource defaults
  resources:
    cpu_limit: "4"
    memory_limit: "8G"
    timeout: "24h"

  # License handling
  license:
    source: "file"  # or "env", "vault"
    path: "~/.config/janus/nonmem.lic"

# Hybrid: Docker execution on SLURM grid
scheduler: "SLURM"
slurm:
  mode: "CLI"
  docker_support: true  # Submit Docker jobs to SLURM
```

### Future Extensions

**Registry Integration**:
- Private container registries
- Automatic image pulling/caching
- Image signing and verification

**Advanced Scheduling**:
- GPU-enabled containers for specialized models
- Multi-container jobs (e.g., preprocessing + execution)
- Container orchestration (Kubernetes backend)

**Enhanced Monitoring**:
- Real-time resource usage metrics
- Container health checks
- Log aggregation and search

## Why This Exists

Because paying excessive monthly license fees to run modeling software in airgapped environments doesn't make sense. Janus provides enterprise-grade functionality at a fraction of the cost.

## Pricing

Janus offers professional NONMEM grid management at a significantly lower cost than traditional solutions, with transparent pricing and no hidden fees.

---

## Executor Binary

Janus includes a lightweight **executor** binary that acts as a standalone container orchestration proxy for pharmacometric modeling tools. The executor provides a drop-in replacement for commands like `nmfe74`, `stan`, or other modeling tool executables, automatically wrapping execution in Hermes-managed containers.

### What is the Executor?

The executor is a supplementary binary that enables containerized execution without requiring the full Janus GUI. It's designed for:

- **CLI-based workflows**: Users who prefer command-line modeling
- **CI/CD pipelines**: Automated model validation and testing
- **Remote execution**: Simple container orchestration on compute nodes
- **Reproducible environments**: Consistent execution across platforms

### Key Features

- **Zero-configuration discovery**: Automatically finds `.janus.config.json` next to model files
- **Transparent argument pass-through**: All arguments (except `--executor-*` flags) pass directly to the containerized tool
- **Execution tracking**: Integrates with Janus run logs for audit trail and reproducibility
- **License verification**: Built-in JWT license validation with embedded public key
- **Streaming output**: Real-time stdout/stderr from containerized execution
- **Signal handling**: Proper SIGINT/SIGTERM forwarding to containers

### Installation

Download the executor binary for your platform from the [releases page](https://github.com/shairozan/janus/releases):

**Linux (AMD64)**:
```bash
chmod +x executor-linux-amd64
sudo mv executor-linux-amd64 /usr/local/bin/executor
```

**Linux (ARM64)**:
```bash
chmod +x executor-linux-arm64
sudo mv executor-linux-arm64 /usr/local/bin/executor
```

**macOS (Intel)**:
```bash
chmod +x executor-darwin-amd64
sudo mv executor-darwin-amd64 /usr/local/bin/executor
```

**macOS (Apple Silicon)**:
```bash
chmod +x executor-darwin-arm64
sudo mv executor-darwin-arm64 /usr/local/bin/executor
```

**Windows**:
```powershell
# Place executor-windows-amd64.exe in a directory on your PATH
# Or run directly: .\executor-windows-amd64.exe
```

### Usage

**Basic usage** (drop-in replacement for NONMEM/Stan/etc.):

```bash
# Instead of: nmfe74 model.mod model.lst
executor model.mod model.lst

# Instead of: stan model.stan --data=data.json
executor model.stan --data=data.json

# Instead of: monolix model.mlxtran
executor model.mlxtran
```

**With executor-specific flags**:

```bash
# Specify Hermes config explicitly
executor --executor-hermes-config=/path/to/.janus.config.json model.mod model.lst

# Quiet mode (suppress stdout/stderr streaming)
executor --executor-quiet model.mod model.lst

# Disable run log updates
executor --executor-no-runlog model.mod model.lst

# Show help
executor --executor-help

# Show version
executor --executor-version
```

### Configuration Discovery

The executor uses a 4-step heuristic to find `.janus.config.json`:

1. **Explicit flag**: `--executor-hermes-config` takes precedence
2. **Model file location**: Scans arguments for file paths, checks for adjacent `.janus.config.json`
3. **Current directory**: Falls back to `.janus.config.json` in working directory
4. **Error with hints**: Provides helpful error message if no config found

### Hermes Configuration for Executor

Create a `.janus.config.json` file next to your model files or in your working directory:

```json
{
  "image": "shairozan/hermes-nonmem:nm76",
  "license": {
    "path": "/home/user/.config/janus/nonmem.lic"
  },
  "resources": {
    "cpus": "4",
    "memory": "8G",
    "timeout": "24h"
  },
  "container": {
    "port": 50051,
    "cleanup": true,
    "startup_timeout": "30s"
  },
  "retain": [
    "*.lst",
    "*.ext",
    "*.tab",
    "*.cov",
    "*.phi"
  ]
}
```

### Requirements

- Docker installed and accessible
- `.janus.config.json` with Hermes container settings
- Valid Janus license JWT (or dev mode build without embedded key)
- Hermes container image (e.g., `shairozan/hermes-nonmem:nm76`)

### Example Workflow

```bash
# 1. Create config next to your model
cd ~/models/project1
cat > .janus.config.json <<EOF
{
  "image": "shairozan/hermes-nonmem:nm76",
  "license": {"path": "~/.config/janus/nonmem.lic"},
  "resources": {"cpus": "4", "memory": "8G"},
  "retain": ["*.lst", "*.ext", "*.tab"]
}
EOF

# 2. Run model with executor (drop-in replacement)
executor run001.mod run001.lst

# 3. Check run log
cat .janus_history.json
```

### Integration with Grid Schedulers

The executor works seamlessly with grid schedulers:

**SLURM**:
```bash
#!/bin/bash
#SBATCH --job-name=model-run
#SBATCH --cpus-per-task=4
#SBATCH --mem=8G

executor model.mod model.lst
```

**SGE**:
```bash
#!/bin/bash
#$ -N model-run
#$ -pe smp 4

executor model.mod model.lst
```

### Building the Executor

Build locally with mage:

```bash
# Build for current platform
mage buildExecutor

# Build for all platforms (Linux amd64/arm64, macOS amd64/arm64, Windows amd64)
mage buildExecutorAll
```

Cross-platform builds are automatically created during GitHub releases.

---

## Run Log Signing (CFR 21 Part 11)

Janus can sign run log records so anyone reading them later can tell whether they
have been altered, and who produced them. Signing is optional — Janus runs fine
without it and records are simply unsigned.

```bash
janus keys generate
```

That creates an RSA-2048 key, stores the private half in your OS credential store
(Keychain, Credential Manager, or Secret Service), and binds an identity to it —
seeded from your `git config user.email` if you have one.

To let colleagues verify your records, send them:

```bash
janus keys export-public
```

They add the output to the keyring at their `signing.keyring_path`. That keyring
is what decides whose signatures they accept; a key it does not list reports as
`Untrusted` rather than being silently accepted.

On headless hosts — servers, containers, CI — there is no credential store, so
point Janus at a key file instead:

```yaml
signing:
  backend: file
  private_key_path: ~/.config/janus/signing.pem
  identity: you@example.com
```

Full details, including key rotation and moving a key between machines, are in
[documentation/features/signing_keys.md](documentation/features/signing_keys.md).

---

## Contributing

Contributions are welcome — particularly from people who actually run models and
hit something that annoyed them.

- [CONTRIBUTING.md](CONTRIBUTING.md) — building, testing, and what reviewers look for
- [SECURITY.md](SECURITY.md) — reporting a vulnerability (please not in a public issue)
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

For anything larger than a small fix, open an issue first. A clean clone builds
with no credentials and no tokens; if it asks you for either, that is a bug.

## License

[MIT](LICENSE). Note that NONMEM itself is licensed commercial software: Janus
neither includes nor redistributes it, and cannot ship a NONMEM container image.
You supply your own — see
[documentation/features/hermes/build-your-own-image.md](documentation/features/hermes/build-your-own-image.md).

---

*"Less hype, more function" - build something that works, adoption will follow.*
