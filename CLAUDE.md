You're helping to build a GUI replacement for Certara Pirana based in golang and the 
fyne.io framework. Primary functionality is about:

1. Loading a model
2. Execution (Locally or Remotely)
3. Tracking execution via log that cohabitates with the model
4. Audit trail (Eventually)
5. Self validating state

# Code Expectations

1. Code should follow idiomatic standards for go
2. Orthogonal architecture should be respected
   3. Nothing should be _constructed_ in lower layers unless a factory pattern is required
   4. Things should be built at the highest layers and handed through layers
5. Errors should not be abandoned or silently handled
6. Global variables are forbidden unless _EXPLICITLY_ allowed here

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
- Audit trail generation

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
- Validate file handling, project management, audit trails
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
- ```type SlurmSummary struct {
    Errors     []SlurmError    `json:"errors"`
    Statistics SlurmStatistics `json:"statistics"`
}

type SlurmStatistics struct {
    PartsPacked            int `json:"parts_packed"`
    RequestTime            int `json:"req_time"`
    RequestTimeStart       int `json:"req_time_start"`
    ServerThreadCount      int `json:"server_thread_count"`
    AgentQueueSize         int `json:"agent_queue_size"`
    AgentThreadCount       int `json:"agent_thread_count"`
    DBDAgentQueueSize      int `json:"dbd_agent_queue_size"`
    GetTimeOfDayLatency    int `json:"get_time_of_day_latency"`
    ScheduleCycleMax       int `json:"schedule_cycle_max"`
    ScheduleCycleLast      int `json:"schedule_cycle_last"`
    ScheduleCycleTotal     int `json:"schedule_cycle_total"`
    ScheduleCycleMean      int `json:"schedule_cycle_mean"`
    ScheduleCycleMeanDepth int `json:"schedule_cycle_mean_depth"`
    ScheduleCyclePerMinute int `json:"schedule_cycle_per_minute"`
    ScheduleQueueLength    int `json:"schedule_queue_length"`
    JobsSubmitted          int `json:"jobs_submitted"`
    JobsStarted            int `json:"jobs_started"`
    JobsCompleted          int `json:"jobs_completed"`
    JobsCancelled          int `json:"jobs_cancelled"`
    JobsFailed             int `json:"jobs_failed"`
    JobsPending            int `json:"jobs_pending"`
    JobsRunning            int `json:"jobs_running"`
    // JobStatesTimestamp is an epoch value for when the above statistics for jobs was collected
    JobStatesTimestamp int `json:"job_states_ts"`

    // Backfill components
    BFBackfilledJobs     int  `json:"bf_backfilled_jobs"`
    BFLastBackfilledJobs int  `json:"bf_last_backfilled_jobs"`
    BFBackfilledHETJobs  int  `json:"bf_backfilled_het_jobs"`
    BFCycleCounter       int  `json:"bf_cycle_counter"`
    BFCycleMean          int  `json:"bf_cycle_mean"`
    BFDepthMean          int  `json:"bf_depth_mean"`
    BFDepthMeanTry       int  `json:"bf_depth_mean_try"`
    BFCycleLast          int  `json:"bf_cycle_last"`
    BFCycleMax           int  `json:"bf_cycle_max"`
    BFQueueLength        int  `json:"bf_queue_len"`
    BFQueueLengthMean    int  `json:"bf_queue_len_mean"`
    BFWhenLastCycle      int  `json:"bf_when_last_cycle"`
    BFActive             bool `json:"bf_active"`
}

type JobCreationRequest struct {
    Account                  *string             `json:"account,omitempty"`
    AccrueTime               *int                `json:"accrue_time,omitempty"`
    AdminComment             *string             `json:"admin_comment,omitempty"`
    ArrayJobId               *int                `json:"array_job_id,omitempty"`
    ArrayTaskId              *int                `json:"array_task_id,omitempty"`
    ArrayMaxTasks            *int                `json:"array_max_tasks,omitempty"`
    AssociationID            *int                `json:"association_id,omitempty"`
    BatchFeatures            *string             `json:"batch_features,omitempty"`
    BatchFlag                *bool               `json:"batch_flag,omitempty"`
    BatchHost                *string             `json:"batch_host,omitempty"`
    Flags                    *[]string           `json:"flags,omitempty"`
    BurstBuffer              *string             `json:"burst_buffer,omitempty"`
    BurstBufferState         *string             `json:"burst_buffer_state,omitempty"`
    Cluster                  *string             `json:"cluster,omitempty"`
    ClusterFeatures          *string             `json:"cluster_features,omitempty"`
    Command                  *string             `json:"command,omitempty"`
    Comment                  *string             `json:"comment,omitempty"`
    Contiguous               *bool               `json:"contiguous,omitempty"`
    BillableTres             *float64            `json:"billable_tres,omitempty"`
    CPUsPerTask              *int                `json:"cpus_per_task,omitempty"`
    CPUFrequencyMinimum      *int                `json:"cpu_frequency_minimum,omitempty"`
    CPUFrequencyMaximum      *int                `json:"cpu_frequency_maximum,omitempty"`
    CPUFrequencyGovernor     *int                `json:"cpu_frequency_governor,omitempty"`
    CPUSPerTres              *string             `json:"cpus_per_tres,omitempty"`
    Deadline                 *int                `json:"deadline,omitempty"`
    DelayBoot                *int                `json:"delay_boot,omitempty"`
    Dependency               *string             `json:"dependency,omitempty"`
    DerivedExitCode          *int                `json:"derived_exit_code,omitempty"`
    EligibleTime             *int                `json:"eligible_time,omitempty"`
    EndTime                  *int                `json:"end_time,omitempty"`
    Environment              map[string]string   `json:"environment" required:"true"`
    ExcludedNodes            *string             `json:"excluded_nodes,omitempty"`
    ExitCode                 *int                `json:"exit_code,omitempty"`
    Features                 *string             `json:"features,omitempty"`
    FederationOrigin         *string             `json:"federation_origin,omitempty"`
    FederationSiblingsActive *string             `json:"federation_siblings_active,omitempty"`
    FederationSiblingsViable *string             `json:"federation_siblings_viable,omitempty"`
    GRESDetail               *[]string           `json:"gres_detail,omitempty"`
    GroupID                  *int                `json:"group_id,omitempty"`
    JobId                    *int                `json:"job_id,omitempty"`
    Resources                *slurm.JobResources `json:"job_resources,omitempty"`
    State                    *string             `json:"job_state,omitempty"`
    LastScheduledEvaluation  *int                `json:"last_sched_evaluation,omitempty"`
    Licenses                 *string             `json:"licenses,omitempty"`
    MaxCPUs                  *int                `json:"max_cpus,omitempty"`
    MaxNodes                 *int                `json:"max_nodes,omitempty"`
    MCSLabel                 *string             `json:"mcs_label,omitempty"`
    MemoryPerTres            *string             `json:"memory_per_tres,omitempty"`
    Name                     *string             `json:"name,omitempty"`
    Nodes                    *string             `json:"nodes,omitempty"`
    Nice                     *string             `json:"nice,omitempty"`
    TasksPerCore             *int                `json:"tasks_per_core,omitempty"`
    TasksPerSocket           *int                `json:"tasks_per_socket,omitempty"`
    TasksPerBoard            *int                `json:"tasks_per_board,omitempty"`
    CPUS                     *int                `json:"cpus,omitempty"`
    NodeCounts               *int                `json:"node_counts,omitempty"`
    Tasks                    *int                `json:"tasks,omitempty"`
    HetJobID                 *int                `json:"het_job_id,omitempty"`
    HetJobIDSet              *string             `json:"het_job_id_set,omitempty"`
    HetJobOffset             *int                `json:"het_job_offset,omitempty"`
    Partition                *string             `json:"partition,omitempty"`
    MemoryPerNode            *int                `json:"memory_per_node,omitempty"`
    MemoryPerCPU             *int                `json:"memory_per_cpu,omitempty"`
    MinimumCPUsPerNod        *int                `json:"minimum_cpus_per_nod,omitempty"`
    MinimumTmpDiskPerNode    *int                `json:"minimum_tmp_disk_per_node,omitempty"`
    PreemptTime              *int                `json:"preempt_time,omitempty"`
    PreSusTime               *int                `json:"pre_sus_time,omitempty"`
    Priority                 *int                `json:"priority,omitempty"`
    Profile                  *string             `json:"profile,omitempty"`
    QOS                      *string             `json:"qos,omitempty"`
    Reboot                   *bool               `json:"reboot,omitempty"`
    RequiredNodes            *string             `json:"required_nodes,omitempty"`
    Requeue                  *bool               `json:"requeue,omitempty"`
    ResizeTime               *int                `json:"resize_time,omitempty"`
    RestartCount             *int                `json:"restart_cnt,omitempty"`
    ResvName                 *string             `json:"resv_name,omitempty"`
    Shared                   *string             `json:"shared,omitempty"`
    ShowFlags                *[]string           `json:"show_flags,omitempty"`
    SocketsPerBoard          *int                `json:"sockets_per_board,omitempty"`
    SocketsPerNode           *int                `json:"sockets_per_node,omitempty"`
    StartTime                *int                `json:"start_time,omitempty"`
    StateDescription         *string             `json:"state_description,omitempty"`
    StandardError            *string             `json:"standard_error,omitempty"`
    StandardInput            *string             `json:"standard_input,omitempty"`
    StandardOutput           *string             `json:"standard_output,omitempty"`
    SubmitTime               *int                `json:"submit_time,omitempty"`
    SuspendTime              *int                `json:"suspend_time,omitempty"`
    TimeLimit                *int                `json:"time_limit,omitempty"`
    TimeMinimum              *int                `json:"time_minimum,omitempty"`
    ThreadsPerCore           *int                `json:"threads_per_core,omitempty"`
    TresBind                 *string             `json:"tres_bind,omitempty"`
    TresFreq                 *string             `json:"tres_freq,omitempty"`
    TresPerJob               *string             `json:"tres_per_job,omitempty"`
    TresPerNode              *string             `json:"tres_per_node,omitempty"`
    TresPerSocket            *string             `json:"tres_per_socket,omitempty"`
    TresPerTask              *string             `json:"tres_per_task,omitempty"`
    TresReqStr               *string             `json:"tres_req_str,omitempty"`
    TresAllocStr             *string             `json:"tres_alloc_str,omitempty"`
    UserID                   *int                `json:"user_id,omitempty"`
    UserName                 *string             `json:"user_name,omitempty"`
    WCKey                    *string             `json:"wc_key,omitempty"`
    CurrentWorkingDirectory  *string             `json:"CurrentWorkingDirectory,omitempty"`
}``` is a known working submission model for v0.0.36. It's worth reviewing as a starting point

# Claude Code File Editing Troubleshooting

## When Edit Tool Continuously Fails

If you encounter repeated failures when trying to edit files with the Edit tool, the most common cause is **whitespace mismatch**. Here's how to diagnose and fix it:

### Diagnosis Steps

1. **Check File Indentation Type**:
   ```bash
   # Show actual whitespace characters (tabs show as ^I, spaces as literal spaces)
   cat -A filename.go | head -20
   ```

2. **Common Issue**: Go files often use **tab characters** for indentation, but when you read them with the Read tool, they display as spaces in the output.

3. **Why Edit Fails**: When you copy text from Read tool output (which shows spaces), and try to match it against the actual file content (which contains tabs), the Edit tool fails because the strings don't match exactly.

### Solution Approaches

**Option 1: Use Exact Characters**
- When editing, use the actual tab characters, not the visual spaces shown in Read output
- Copy the **exact** whitespace from `cat -A` output or similar tools

**Option 2: Alternative Edit Methods**
- Use `sed` for simple replacements: `sed -i 's/old/new/g' file.go`
- Use `MultiEdit` tool which may handle whitespace better
- For complex changes, use `Write` tool to rewrite sections

**Option 3: Debugging Pattern**
```bash
# 1. Examine the exact file content around your target area
grep -n "your search text" file.go
sed -n '20,30p' file.go | cat -A  # Show lines 20-30 with whitespace visible

# 2. Make targeted changes with sed if Edit fails
sed -i 's/exact_old_text/exact_new_text/g' file.go

# 3. Verify the change worked
grep -A5 -B5 "new_text" file.go
```

### Example: Tab vs Space Issue

**What Read tool shows you**:
```
    if condition {
        doSomething()
    }
```

**What's actually in the file** (when using tabs):
```
^Iif condition {^I^I// ^I = tab character
^I^IdoSomething()
^I}
```

**Failed Edit** (trying to match spaces):
```go
// This fails because file contains tabs, not spaces
Edit("    if condition {", "    if newCondition {")
```

**Successful Edit** (using actual tabs):
```go
// This works because it matches the actual tab characters
Edit("\\tif condition {", "\\tif newCondition {")
```

### Prevention

- When copying text from Read tool output for Edit operations, be aware that indentation may be displayed differently than stored
- For complex multi-line edits, consider using MultiEdit or breaking into smaller single-line changes
- When in doubt, use `cat -A filename` to see the exact characters before attempting edits

### Real Example from This Codebase

When trying to edit `internal/execution/slurm.go`:

**What failed**:
```go
Edit("	// Wait for file collection to complete\n	wg.Wait()\n\n	duration := time.Since(startTime)", "...")
```

**What worked**:
```go
Edit("	// Wait for file collection to complete\n	wg.Wait()\n\n	duration := time.Since(startTime)", "...")
// Using actual tab characters (^I) instead of visual spaces
```

The key was using `cat -A` to see that the file used tabs (`^I`) not spaces, then matching the exact characters.

# Docker-Based Development (Windows CGO Solution)

Building Go applications with CGO dependencies (like Fyne GUI framework) on Windows can be problematic due to C compiler toolchain issues. To solve this, use the Docker-based mage commands that leverage the `dukeofubuntu/janus-ci:latest-ubuntu24` image.

## Docker Commands Reference

All Docker commands are now organized under the `docker:` namespace for better organization.

### Building
- `mage docker:build` - Build the binary in Docker (solves Windows CGO issues)

### Testing
- `mage docker:test` - Run all tests in Docker
- `mage docker:unit` - Run unit tests only in Docker
- `mage docker:integration` - Run integration tests in Docker
- `mage docker:signalTest` - Run signal handling tests in Docker
- `mage docker:mockSlurm` - Run mock SLURM tests in Docker

### Development Workflows
- `mage docker:check` - Run format, lint, and unit tests in Docker
- `mage docker:checkAll` - Run complete validation (format, lint, tests, build) in Docker
- `mage docker:lint` - Run linting in Docker

### Specialized Operations
- `mage docker:validation` - Run validation tests in Docker
- `mage docker:release v1.0.0` - Build release version in Docker

### Debugging
- `mage docker:shell` - Open interactive shell in Docker container for debugging

## When to Use Docker Commands (Windows Primary OS)

**ALWAYS use Docker commands on Windows for:**
1. Building the application (`mage docker:build` instead of `mage build`)
2. Running GUI-related tests (anything in `internal/gui/`)
3. Running integration tests that might have CGO dependencies
4. Running the complete validation pipeline (`mage docker:checkAll`)

**Can use local commands on Windows for:**
- Pure Go unit tests (no CGO dependencies)
- Linting Go code (if golangci-lint is installed locally)
- File operations and code analysis

## Example Workflow for Windows Development

```bash
# Check code quality (format, lint, unit tests, build)
mage docker:checkAll

# Run specific tests
mage docker:signalTest
mage docker:mockSlurm

# Build for release
mage docker:build

# Debug issues
mage docker:shell
# (then inside container: mage build, go test -v ./..., etc.)
```

## Prerequisites

- Docker Desktop installed and running
- `dukeofubuntu/janus-ci:latest-ubuntu24` image available:
  ```bash
  docker pull dukeofubuntu/janus-ci:latest-ubuntu24
  ```

## How It Works

The Docker commands:
1. Mount the current directory as `/workspace` in the container
2. Run mage commands inside the Linux container with proper CGO toolchain
3. Output built artifacts back to the Windows host
4. Provide identical behavior to local commands but in a controlled Linux environment

This approach eliminates Windows CGO compilation issues while maintaining the exact same development workflow.