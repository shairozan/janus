# Executor Binary - Developer Documentation

## Overview

The `executor` binary is a lightweight, standalone container orchestration proxy designed to provide transparent drop-in replacement functionality for pharmacometric modeling tools (NONMEM, Stan, Torsten, Monolix, etc.). It reuses Janus's Hermes execution infrastructure but operates independently as a minimal, zero-dependency CLI tool.

## Design Philosophy

### Core Principle: Maximally Transparent Pass-Through

The executor does **not** interpret tool-specific arguments. Instead:

1. Finds the `.janus.config.json` configuration file
2. Passes ALL arguments (except `--executor-*` flags) to the container verbatim
3. Streams output back to the user in real-time
4. Writes artifacts to the model directory
5. Updates run log (if present)

The container image specified in `.janus.config.json` determines what tool runs and how it behaves.

### Key Design Decisions

- ✅ **No Cobra/Viper** - Manual argument parsing only (standard library)
- ✅ **No external CLI frameworks** - Minimal dependencies
- ✅ **Single binary** - Statically linked, portable
- ✅ **Pure Go (CGO disabled)** - Easy cross-compilation for all platforms
- ✅ **Zero argument interpretation** - Pass everything through except `--executor-*`
- ✅ **Heuristic config discovery** - Smart path-based discovery
- ✅ **Fail fast** - License and config validation before Docker operations

## Architecture

### File Structure

```
cmd/executor/
├── main.go          # Entry point and orchestration
├── args.go          # Argument parsing (executor vs container args)
├── args_test.go     # Argument parsing tests
├── config.go        # Config discovery algorithm
├── config_test.go   # Config discovery tests
├── execute.go       # Hermes execution orchestration
├── help.go          # Help and version display
└── license.go       # License verification
```

### Component Responsibilities

#### main.go
- Entry point and high-level orchestration
- Coordinates the execution flow:
  1. Parse arguments
  2. Handle `--executor-help` and `--executor-version`
  3. Find config file
  4. Load Hermes configuration
  5. Verify license
  6. Execute via Hermes

#### args.go
- Manual argument parser (no Cobra)
- Separates executor-specific flags from container arguments
- Executor flags use `--executor-` prefix to avoid collisions
- Supports both `--flag=value` and `--flag value` syntax

**Executor Flags:**
- `--executor-help` - Show executor help
- `--executor-version` - Show version info
- `--executor-license PATH` - License JWT file path
- `--executor-quiet` - Suppress output streaming
- `--executor-hermes-config PATH` - Explicit config file location
- `--executor-no-runlog` - Skip run log update

#### config.go
- Config discovery algorithm (4-step heuristic)
- Finds `.janus.config.json` via:
  1. Explicit `--executor-hermes-config` flag
  2. Scan arguments for file paths, check adjacent directories
  3. Fall back to current working directory
  4. Error with helpful hints

#### execute.go
- Hermes execution orchestration
- Integrates with existing Janus execution infrastructure:
  - `internal/execution/hermes_payload.go::BuildHermesPayload()`
  - `internal/execution/hermes.go::NewHermesExecutor()`
  - `internal/execution/interface.go::StreamingExecutor`
- Handles signal forwarding (SIGINT/SIGTERM)
- Streams output to stdout/stderr by default
- Collects and writes artifacts back to model directory

#### license.go
- License verification using embedded public key
- Embeds public key via `//go:embed` directive
- Validates JWT signature, expiration, claims
- Fails fast before any Docker operations

#### help.go
- Help text display (`--executor-help`)
- Version information (`--executor-version`)
- Usage examples and flag documentation

## Development Workflow

### Building

**Build for current platform:**
```bash
mage buildExecutor
```

**Build for all platforms:**
```bash
mage buildExecutorAll
```

**Manual build:**
```bash
go build -ldflags "-s -w \
  -X github.com/pharmalytica/janus/internal/version.Version=dev \
  -X github.com/pharmalytica/janus/internal/version.Commit=$(git rev-parse HEAD) \
  -X github.com/pharmalytica/janus/internal/version.Date=$(date -u '+%Y-%m-%dT%H:%M:%SZ') \
  -X github.com/pharmalytica/janus/internal/version.BuiltBy=local" \
  -o executor ./cmd/executor
```

### Testing

**Run unit tests:**
```bash
go test ./cmd/executor/...
```

**Run with coverage:**
```bash
go test -cover ./cmd/executor/...
```

**Test argument parsing:**
```bash
go test -v -run TestArgumentParsing ./cmd/executor/
```

**Test config discovery:**
```bash
go test -v -run TestConfigDiscovery ./cmd/executor/
```

### Local Testing

**Create test configuration:**
```bash
cat > /tmp/test-model/.janus.config.json <<EOF
{
  "image": "pharmalytica/hermes-nonmem:nm76",
  "license": {"path": "~/.config/janus/nonmem.lic"},
  "resources": {"cpus": "4", "memory": "8G"},
  "retain": ["*.lst", "*.ext", "*.tab"]
}
EOF
```

> `retain` is authoritative per-model (canonically nested under the `hermes`
> key); when empty/absent it inherits the global `hermes.retain`, then the engine
> category defaults. `janus hermes init` seeds it with the NONMEM best-practice set.

**Test executor:**
```bash
./executor --executor-help
./executor --executor-version
./executor /tmp/test-model/model.mod /tmp/test-model/model.lst
```

## Code Patterns

### Argument Parsing Pattern

```go
// parseExecutorFlags separates executor flags from container arguments
func parseExecutorFlags(args []string) (ExecutorFlags, []string) {
    flags := ExecutorFlags{}
    containerArgs := []string{}

    for i := 0; i < len(args); i++ {
        arg := args[i]

        switch {
        case arg == "--executor-help":
            flags.Help = true
        case strings.HasPrefix(arg, "--executor-license="):
            flags.License = strings.TrimPrefix(arg, "--executor-license=")
        case arg == "--executor-license":
            if i+1 < len(args) {
                flags.License = args[i+1]
                i++ // Skip next arg (value)
            }
        default:
            // Not an executor flag - pass to container
            containerArgs = append(containerArgs, arg)
        }
    }

    return flags, containerArgs
}
```

### Config Discovery Pattern

```go
// Strategy 1: Explicit config takes precedence
if explicitConfigPath != "" {
    return explicitConfigPath, "", nil
}

// Strategy 2: Scan arguments for file paths
for _, arg := range args {
    fileInfo, err := os.Stat(arg)
    if err != nil {
        continue
    }

    searchDir := filepath.Dir(arg)
    configPath := filepath.Join(searchDir, ".janus.config.json")
    if _, err := os.Stat(configPath); err == nil {
        return configPath, arg, nil
    }
}

// Strategy 3: Fall back to current directory
cwd, _ := os.Getwd()
configPath := filepath.Join(cwd, ".janus.config.json")
if _, err := os.Stat(configPath); err == nil {
    return configPath, "", nil
}

// Strategy 4: Error with hints
return "", "", fmt.Errorf("no .janus.config.json found")
```

### Hermes Integration Pattern

```go
// Build Hermes payload (reuse functional core)
payload, err := execution.BuildHermesPayload(modelPath, hermesConfig)
if err != nil {
    return 1
}

// Override entry point args with ALL container args
payload.EntryPointArgs = args

// Create Hermes executor
executor := execution.NewHermesExecutor(nil, hermesConfig)

// Setup output streaming
if !flags.Quiet {
    if streamExec, ok := executor.(execution.StreamingExecutor); ok {
        streamExec.SetOutputWriters(os.Stdout, os.Stderr)
    }
}

// Execute and get exit code
result, err := executor.Execute(ctx, modelPath)
return result.ExitCode
```

## Dependencies

### Internal Janus Packages (Reused)
- `internal/config` - Configuration management
- `internal/execution` - Hermes execution infrastructure
- `internal/license` - License verification
- `internal/version` - Version information

### External Dependencies
- **Standard library only** - No external CLI frameworks
- Go 1.21+ (for standard library features)

### Build-time Dependencies
- `//go:embed` for embedding license public key
- `ldflags` for version injection

## Release Process

The executor is built and released automatically via GitHub Actions:

1. **Trigger**: Push git tag (`v*.*.*`)
2. **Build matrix**: Builds for 5 platforms
   - Linux AMD64
   - Linux ARM64
   - Windows AMD64
   - macOS AMD64 (Intel)
   - macOS ARM64 (Apple Silicon)
3. **Artifact upload**: Binaries uploaded to release
4. **Release notes**: Auto-generated with installation instructions

See [.github/workflows/release.yml](../../.github/workflows/release.yml) for details.

## Common Development Tasks

### Adding a New Executor Flag

1. Update `ExecutorFlags` struct in `args.go`
2. Add parsing logic in `parseExecutorFlags()`
3. Add tests in `args_test.go`
4. Update help text in `help.go`
5. Implement flag behavior in `execute.go` or `main.go`

### Debugging Config Discovery

```bash
# Enable verbose output (add to config.go temporarily)
fmt.Fprintf(os.Stderr, "DEBUG: Checking config at: %s\n", configPath)

# Or use strace/dtrace to see filesystem access
strace -e openat ./executor model.mod 2>&1 | grep janus.config.json
```

### Testing License Verification

```bash
# Build with embedded public key
cp .license_public_key.pem ./cmd/executor/
go build ./cmd/executor

# Test with valid license
./executor --executor-license=/path/to/valid.jwt model.mod

# Test with invalid license (should fail)
./executor --executor-license=/path/to/invalid.jwt model.mod
```

## Design Rationale

### Why No Cobra/Viper?

- **Minimal binary size**: Cobra/Viper add ~3MB to binary
- **Simple requirements**: Only need flag parsing, not complex subcommands
- **Dependency hygiene**: Standard library only = easier maintenance
- **Performance**: Manual parsing is faster for simple cases

### Why Heuristic Config Discovery?

- **User convenience**: No need to specify config every time
- **Drop-in replacement**: Matches NONMEM/Stan usage patterns
- **Explicit override**: `--executor-hermes-config` for advanced users
- **Fail-safe**: Clear error messages when config not found

### Why Embedded Public Key?

- **Zero setup**: No need to distribute separate key file
- **Security**: Binary is self-contained, validates licenses independently
- **Simplicity**: Users don't need to manage key files

## Troubleshooting

### Common Issues

**Issue: Config not found**
```
Error: no .janus.config.json found
```
**Solution**: Create config in model directory or use `--executor-hermes-config`

**Issue: License verification failed**
```
License verification failed: invalid signature
```
**Solution**: Ensure license JWT is valid and matches embedded public key

**Issue: Container not found**
```
Error: image not found: pharmalytica/hermes-nonmem:nm76
```
**Solution**: Pull container image: `docker pull pharmalytica/hermes-nonmem:nm76`

**Issue: Permission denied (Docker)**
```
Error: permission denied while trying to connect to Docker daemon
```
**Solution**: Add user to docker group or use sudo

## Further Reading

- [Technical Implementation Plan](../../documentation/features/supplementary_binaries/technical_implementation_plan.md)
- [User Guide](../../documentation/features/supplementary_binaries/user_guide.md)
- [Integration Examples](../../documentation/features/supplementary_binaries/integration_examples.md)
- [Main README](../../README.md#executor-binary)
