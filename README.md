# Janus: NONMEM Grid Management Tool

A modern, cost-effective replacement for Certara Pirana using Go + Fyne.io, with pluggable orchestrator backends and built-in CFR 21 Part 11 compliance.

## Project Overview

**Goal**: Build a GUI tool that doesn't require monthly license validation or punch holes in your firewall just to submit modeling jobs.

**Architecture**: Clean separation between GUI, orchestrator abstraction layer, and backend implementations (starting with PSN, expanding to BBI).

## Core User Journeys

1. **Submit Jobs**: Modelers can submit NONMEM jobs to SLURM/SGE grids with proper validation
2. **View Jobs**: See only your own job details (filtered by user ownership)
3. **Cancel Jobs**: Cancel only jobs you own (built-in authorization)
4. **Execution History**: Browse personal job history for record-keeping and command reuse
5. **Generate IQ/OQ**: Sysadmins can auto-generate validation documentation

## Development Setup (Windows)

### Prerequisites

You need Go 1.23+ and a C compiler for CGO (Fyne requirement).

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
4. **Single execution channel** - All job submissions flow through one message bus for audit/hooks

# Developer Experience

This was originally developed purely on linux, the moved to windows, which exposed how clunky windows can be. The mage architecture has several commands added to help this _still work_. `run dockerlint` will run lint in a container (the same that CI is using) to check for linting rules

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
audit-engine: true
projects: true

# Validation Configuration for CFR 21 Part 11 compliance
validation:
  iq: "~/.config/janus/validation/iq-report.json"
  oq: "~/.config/janus/validation/oq-report.json"
```

## Current Status

- ✅ Basic GUI structure (tabs, layout, placeholders)
- ✅ Windows development environment setup
- ✅ Run configurations for GoLand
- ✅ SLURM REST API integration with setup wizard
- ⏳ Service layer (execution channels, audit engine)
- ⏳ Project management
- ⏳ MCP server for AI integration

## Future Plans

- **Projects**: Define model/data/metadata, track run history
- **Audit engine**: Pluggable backends (filesystem, PostgreSQL, S3)
- **External hooks**: Environment variable export + script execution
- **MCP server**: Query run history via AI chat
- **Multi-platform**: Linux primary, Windows/Mac secondary
- **Licensing Integration**: OIDC-based enterprise licensing system
- **Supply Chain Security**: GitHub attestations (build provenance, SBOM) for enterprise trust and compliance

## Licensing Architecture (Future Implementation)

### Overview
The setup wizard provides an ideal foundation for enterprise licensing integration. The same UI pattern used for initial configuration can seamlessly handle license activation and validation.

### OIDC Integration Flow

**Architecture**: Browser-based OIDC flow with localhost callback handling

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Setup Wizard   │    │ License Service │    │ Local Listener  │
│                 │    │   (OIDC IdP)    │    │  (localhost)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                        │                        │
         │ 1. Request License     │                        │
         ├────────────────────────┤                        │
         │                        │                        │
         │ 2. Open Browser        │                        │
         │    + Start Listener    │                        │
         ├────────────────────────┤                        │
         │                        │ 3. OIDC Auth Flow     │
         │                        ├────────────────────────┤
         │                        │                        │
         │                        │ 4. Callback + Token   │
         │                        ├────────────────────────┤
         │                        │                        │
         │ 5. License Details     │                        │
         ├────────────────────────┤                        │
         │                        │                        │
         │ 6. Store License       │                        │
         │    + Continue Setup    │                        │
```

### Implementation Components

**1. License Service Integration**
```go
type LicenseClient struct {
    BaseURL      string
    ClientID     string
    ClientSecret string
    RedirectURI  string // http://localhost:8080/callback
}

func (lc *LicenseClient) InitiateLicenseFlow() (*LicenseRequest, error)
func (lc *LicenseClient) StartCallbackListener() (<-chan *License, error)
```

**2. Setup Wizard Enhancement**
- Add "License Type" selection (Community/Enterprise)
- Enterprise path triggers OIDC flow
- Community path continues with current setup
- License details stored in config alongside other settings

**3. Localhost Callback Handler**
```go
type CallbackServer struct {
    Port     int    // Default: 8080
    Path     string // Default: /callback
    Timeout  time.Duration
}

func (cs *CallbackServer) Listen() (<-chan *AuthResult, error)
```

### Configuration Integration

**Enhanced Config Structure**:
```yaml
# User Configuration
organization: "Acme Pharmaceuticals"
default-directory: "~/models"
nonmem-path: "/opt/NONMEM/nm76/run"

# License Configuration (Enterprise only)
license:
  type: "enterprise"           # or "community"
  organization-id: "uuid-here"
  expires: "2025-12-31"
  features: ["audit", "projects", "mcp"]
  token: "jwt-token-here"
```

### Security Considerations

**Token Storage**:
- JWT tokens stored in config file (user readable only)
- Refresh tokens handled automatically
- Graceful degradation on license expiry

**Network Security**:
- Localhost listener only (127.0.0.1 binding)
- Single-use callback URLs
- PKCE (Proof Key for Code Exchange) for security
- Timeout-based listener cleanup

**Airgapped Environments**:
- Offline license validation option
- License file import mechanism
- Manual activation codes for restricted networks

### User Experience

**Setup Flow**:
1. User runs `janus gui` (first time)
2. Setup wizard appears
3. User selects "Enterprise License"
4. Browser opens to license service
5. User authenticates via OIDC
6. License details auto-populate in wizard
7. User completes setup normally
8. Main application launches with full features

**Benefits**:
- No manual license key entry
- Automatic organization detection
- Seamless integration with existing identity providers
- Familiar OAuth flow for enterprise users
- Zero firewall configuration needed

### Future Extensions

**License Management**:
- License status in settings panel
- Usage metrics collection (with consent)
- Feature flag management based on license tier
- Automatic renewal notifications

**Multi-tenancy**:
- Organization-specific configurations
- User-based feature access
- Audit trail integration with license events

## Considerations for Cleanup

NONMEM executions generate numerous temporary and output files. Understanding which to keep is critical for storage management and audit compliance.

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
- `*.janus_history.json` - Execution audit trail (compliance)

### Storage Strategy

**Traditional Approach (BBI/PSN):**
- Create subdirectories for each run
- Archive entire directory trees
- Results in significant storage overhead

**Janus Approach (Implemented):**
- Gzip + Base64 compression for all text fields (stdout, stderr, descriptions)
- Essential output files (`.lst`, `.ext`, `.phi`, `.xml`, `.mod`) embedded directly into audit trail
- Automatic compression on run completion
- **56-98% storage reduction** depending on content (typical: 60% overall)
- Single source of truth for compliance and analysis
- Enables centralized audit database (PostgreSQL, S3) without filesystem dependencies
- Backward compatible: reads both compressed and legacy uncompressed records

**Benefits:**
- Complete audit trail with embedded results
- No orphaned files after cleanup
- Easy migration between storage backends
- Simplified backup procedures
- Cloud-friendly (no need to preserve filesystem structure)

### Cleanup Workflow (Future)

```bash
# After successful execution
janus cleanup --run-id <id> --keep-essential

# Options:
#   --keep-essential: Keep only .mod, .csv, audit trail (embedded results)
#   --keep-diagnostics: Also keep .grd, .shk, .cpu files
#   --dry-run: Show what would be deleted
```

The audit trail becomes the single source of truth, containing:
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

## Why This Exists

Because paying excessive monthly license fees to run modeling software in airgapped environments doesn't make sense. Janus provides enterprise-grade functionality at a fraction of the cost.

## Pricing

Janus offers professional NONMEM grid management at a significantly lower cost than traditional solutions, with transparent pricing and no hidden fees.

---

## Building with License Key

Janus requires a license server public key to be embedded at build time for license validation. The key is embedded using Go's `//go:embed` directive.

### Setup

Place your license server public key at the project root:

```bash
cp /path/to/master_public_key.pem .license_public_key.pem
```

### Building

```bash
mage build
```

The build process will automatically detect `.license_public_key.pem` and embed it via `go:embed`.

### Verifying the Embedded Key

```bash
strings ./janus | grep "BEGIN PUBLIC KEY"
```

### CI/CD Pipeline

The GitHub Actions workflows automatically write the public key from the `LICENSE_PUBLIC_KEY` repository secret to `.license_public_key.pem`, which is then embedded via `go:embed` during build.

---

*"Less hype, more function" - build something that works, adoption will follow.*
