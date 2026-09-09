You're helping to build a GUI replacement for Certara Pirana based in golang and the 
fyne.io framework. Primary functionality is about:

1. Loading a model
2. Execution (Locally or Remotely)
3. Tracking execution via log that cohabitates with the model
4. Execution run log (implemented)
5. Self validating state

# Code Expectations

1. Code should follow idiomatic standards for go
2. Orthogonal architecture should be respected
   3. Nothing should be _constructed_ in lower layers unless a factory pattern is required
   4. Things should be built at the highest layers and handed through layers
5. Errors should not be abandoned or silently handled
6. Global variables are forbidden unless _EXPLICITLY_ allowed here

## GitHub Operations

Prefer the GitHub MCP server tools (`mcp__github__*`) for issues, PRs, and
milestones. The `gh` CLI is allowed and is the right tool for things the MCP
server doesn't cover — most notably **Actions/workflow run and job logs**
(e.g. `gh run view <run-id> --log-failed`, `gh run view --job <job-id> --log`).

## Linting expectations
Follow rules defined in golangci.yml when possible:

1. New lines before returns
2. Unused parameters should be removed
    1. If they're part of an interface signature, just `_` them to keep the signature


# CI
The whole point of this is two things:

1. Testing
2. Artifact generation

If anything you plan to do removes either of the above, it's not an acceptable option.
There's no point to automation if it can't handle releases and the like.

# Docker Development Workflow

## Fast Local Development with Cached Dependencies

Janus Docker commands use a local development container with cached dependencies by default, providing much faster builds and tests compared to the CI image.

### Standard Docker Commands (Fast, with Cached Dependencies)

**First-time setup:**
```bash
mage docker:buildDev  # Build local dev image with cached Go modules (takes 5-10 minutes)
```

**Fast development commands (use dev container by default):**
- `mage docker:build` - Build using cached dev image
- `mage docker:unit` - Run unit tests using cached dev image
- `mage docker:integration` - Run integration tests using cached dev image
- `mage docker:validation` - Run validation tests using cached dev image
- `mage docker:gui` - Run GUI tests using cached dev image
- `mage docker:check` - Run format, lint, and unit tests using cached dev image
- `mage docker:checkAll` - Run complete validation using cached dev image
- `mage docker:shell` - Open interactive shell in dev container

**Maintenance commands:**
- `mage docker:rebuildDev` - Rebuild dev image (after go.mod changes)
- `mage docker:cleanDev` - Remove local dev image

**CI/Debugging commands:**
- `mage docker:ciShell` - Open shell in CI image (for debugging CI issues)

## When to Use Which Commands

### Use Standard Docker Commands When:
- Active local development (fast with cached dependencies)
- Running tests during development
- Building on Windows (solves CGO issues)
- Need fast feedback loops

### Use CI Shell When:
- Debugging CI/CD pipeline issues
- Need exact CI environment replication
- Testing with fresh dependencies

## Speed Comparison

| Command | CI Image (Fresh) | Docker Commands (Cached) | Speedup |
|---------|------------------|-------------------------|---------|
| Unit Tests | ~60-90 seconds | ~15-20 seconds | 3-4x faster |
| Integration Tests | ~120-180 seconds | ~30-45 seconds | 3-4x faster |
| Build | ~45-60 seconds | ~10-15 seconds | 4-5x faster |

## How It Works

The development container (`janus-dev:latest`) pre-caches:
- Go module dependencies from `go.mod`/`go.sum`
- Build artifacts and compilation cache
- Common development tools (golangci-lint v2.1.5, goimports, mage)
- Test dependency compilation

The container only needs to be rebuilt when:
- `go.mod` or `go.sum` changes
- You want to update tool versions
- Base CI image is updated

## Windows CGO Issues Solved

Both Docker approaches solve Windows CGO compilation problems by:
- Using Linux environment with proper CGO toolchain
- Pre-installed build dependencies
- Consistent build environment across development machines 

# Cobra Command Design Pattern

## Functional Command Design (No Init Functions)

This codebase follows a functional approach to Cobra command design that avoids `init()` functions, which are idiomatically discouraged in Go. Instead, commands are created through factory functions that return fully configured cobra.Command instances.

## Core Design Principles

### 1. Factory Functions Return Pre-Configured Commands
Each command package exports a `Command()` function that returns a fully configured `*cobra.Command`:

```go
func Command() *cobra.Command {
    var configuration *config.Config

    c := &cobra.Command{
        Use:   "mycommand",
        Short: "Description",
        PersistentPreRunE: config.NewInitializer(&configuration, config.InitializerOptions{
            ConfigFlagName: "config",
        }),
    }

    attributes(c)
    c.AddCommand(subcommand.Command())

    return c
}
```

### 2. Dependency Injection Through Closures
Dependencies are captured in closures, allowing each command to have its own isolated configuration and state:

```go
func Command() *cobra.Command {
    var cfg *config.Config  // Command-scoped configuration

    return &cobra.Command{
        PreRunE: func(c *cobra.Command, args []string) error {
            // Initialize cfg specific to this command
            cfg, err = config.Process()
            return err
        },
        RunE: func(c *cobra.Command, args []string) error {
            // Use cfg that was initialized in PreRunE
            return doWork(cfg)
        },
    }
}
```

### 3. Configuration Initialization Pattern
Use the generic `config.NewInitializer()` function to handle configuration setup:

```go
PersistentPreRunE: config.NewInitializer(&configuration, config.InitializerOptions{
    ConfigFlagName: "config",
}),
```

This initializer:
- Reads config files and flags
- Handles missing config files gracefully
- Assigns the processed config to the provided pointer
- Works with Viper for configuration management

### 4. Attribute Separation
Keep flag and attribute setup in separate functions for clarity:

```go
func attributes(c *cobra.Command) {
    c.PersistentFlags().String("config", "", "config file")
    c.PersistentFlags().String("organization", "Default Org", "organization name")

    // Bind all flags to viper for configuration override support
    viper.BindPFlags(c.PersistentFlags())
}
```

### 5. Command Composition
Build command hierarchies by calling other `Command()` factory functions:

```go
func Command() *cobra.Command {
    c := &cobra.Command{...}

    c.AddCommand(gui.Command())      // Add subcommand
    c.AddCommand(version.Command())  // Add another subcommand

    return c
}
```

## Benefits of This Pattern

1. **No Global State**: Each command gets its own configuration instance
2. **Testable**: Factory functions can be called in tests with different configurations
3. **No Init Race Conditions**: Avoids the unpredictable order of init() execution
4. **Clear Dependencies**: Dependencies are explicitly passed or captured in closures
5. **Flexible**: Commands can have different initialization logic without affecting others
6. **Composable**: Commands can be easily combined and reused

## Anti-Patterns to Avoid

- Don't use `init()` functions for command setup
- Don't rely on global variables for command state
- Don't use `cobra.OnInitialize()` - handle initialization in PreRunE instead
- Avoid package-level command variables that are mutated during init

## Command Structure Template

```go
package mycommand

import (
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "myapp/internal/config"
)

func Command() *cobra.Command {
    var cfg *config.Config

    cmd := &cobra.Command{
        Use:   "mycommand",
        Short: "Brief description",
        Long:  "Detailed description",
        PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{
            ConfigFlagName: "config",
        }),
        RunE: func(c *cobra.Command, args []string) error {
            // Command implementation using cfg
            return nil
        },
    }

    attributes(cmd)

    return cmd
}

func attributes(c *cobra.Command) {
    // Define flags
    c.Flags().String("flag", "default", "description")

    // Bind to viper for config file override support
    viper.BindPFlags(c.Flags())
}
```

This pattern ensures clean, testable, and maintainable CLI applications with proper separation of concerns and no reliance on global state or init functions.

# Validation Strategy (IQ/OQ)

## Scope and Boundary Definition

Janus validation focuses on **system call generation and output parsing** rather than full end-to-end execution of external tools. This approach provides comprehensive validation while maintaining clear responsibility boundaries.

### Clear Scope Boundaries

**Janus Responsibility (What We Validate):**
- Configuration management and translation
- User interface functionality
- Command construction logic
- File handling and project management
- Grid system integration (command generation + output parsing)
- Execution run log generation

**External Dependencies (What We Don't Validate):**
- NONMEM/PSN mathematical correctness
- Grid scheduler installation/configuration
- Network connectivity to compute resources
- Platform-specific execution environments

## Validation Approach

### 1. NONMEM/PSN Execution Validation

Validate that Janus correctly translates user inputs into proper system calls without requiring actual NONMEM execution:

```go
// Example: Validate command generation
func TestNONMEMCommandGeneration(t *testing.T) {
    config := &config.Config{
        NonmemPath: "/opt/NONMEM/nm76/run/nmfe76",
    }

    executor := execution.NewNONMEMExecutor(config)
    binary, args, err := executor.BuildCommand("model.mod", true, 4, false, []string{"-maxeval=9999"})

    // Validate the EXACT system call that would be made
    assert.NoError(t, err)
    assert.Equal(t, "/opt/NONMEM/nm76/run/nmfe76", binary)
    assert.Equal(t, []string{"model.mod", "model.lst", "-maxeval=9999"}, args)
}
```

### 2. Grid System Validation

Focus on the two critical interfaces where Janus interacts with grid systems:

**Command Generation → Grid System:**
```go
func TestSLURMCommandGeneration(t *testing.T) {
    config := &config.Config{Scheduler: "SLURM"}

    command, args := buildSLURMCommand("model.mod", 4, "2:00:00")

    assert.Equal(t, "sbatch", command)
    assert.Contains(t, args, "--cpus-per-task=4")
    assert.Contains(t, args, "--time=2:00:00")
    assert.Contains(t, args, "--job-name=janus-model")
}
```

**Grid Output → Janus Parsing:**
```go
func TestSLURMOutputParsing(t *testing.T) {
    slurmOutput := "Submitted batch job 12345\n"

    jobID, err := parseSLURMSubmissionOutput(slurmOutput)

    assert.NoError(t, err)
    assert.Equal(t, "12345", jobID)
}
```

## Grid Systems Coverage

### Supported Schedulers
- **SLURM**: `sbatch`, `squeue`, `scancel` command generation and output parsing
- **SGE**: `qsub`, `qstat`, `qdel` command generation and output parsing
- **TORQUE**: `qsub`, `qstat`, `qdel` command generation and output parsing

### Validation Points
✅ Configuration → Correct scheduler selection
✅ Job Submission → Proper command construction
✅ Status Monitoring → Accurate parsing of grid output
✅ Job Control → Cancel/modify commands
✅ Resource Requests → CPU/memory/time parameters
✅ File Staging → Input/output file handling

## IQ/OQ Implementation

### Installation Qualification (IQ)
- Verify Janus installs correctly
- Verify configuration management works
- Verify UI components function
- Verify command construction logic
- Verify grid scheduler detection

### Operational Qualification (OQ)
- Test: "Given config X, does Janus generate system call Y?"
- Test: "Given UI input Z, does Janus produce expected command?"
- Test: "Given grid output A, does Janus parse status B correctly?"
- Validate file handling, project management, run logs
- Validate error conditions and graceful failure handling

## Benefits of This Strategy

1. **Regulatory Compliance**: Each test maps to specific requirements
2. **Deterministic**: System call generation is deterministic and testable
3. **Isolated**: Failures are clearly Janus vs. external tool issues
4. **Efficient**: Fast validation without heavyweight external dependencies
5. **Maintainable**: Tests don't break when external tool versions change
6. **Traceable**: Clear mapping from requirements to validation tests

## Mock Strategy

Use mock executors to simulate external system responses:

```go
type MockGridExecutor struct {
    expectedCommands []string
    mockResponses    []string
}

func (m *MockGridExecutor) Execute(cmd string, args []string) (string, error) {
    // Record what command was attempted
    fullCmd := fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
    m.expectedCommands = append(m.expectedCommands, fullCmd)

    // Return predetermined mock response
    if len(m.mockResponses) > 0 {
        response := m.mockResponses[0]
        m.mockResponses = m.mockResponses[1:]
        return response, nil
    }

    return "", nil
}
```

This validation approach ensures comprehensive testing of Janus functionality while maintaining clear boundaries with external systems, making it ideal for both development and regulatory validation requirements.