// Package scheduler models workload managers (SLURM, SGE, Torque, …) as data —
// a Profile describing the commands to submit/poll/cancel jobs, the templates
// for their arguments, and the regexes for parsing their text output.
//
// This generalizes the previously SLURM-only, hardcoded grid logic so that SGE
// and Torque become first-class instead of falling back to local execution, and
// so that users can override or add scheduler definitions via configuration.
//
// The package is intentionally free of any config/Fyne dependency: command
// generation (render.go) and output parsing (parse.go) are pure and unit-tested
// per the IQ/OQ "validate system-call generation and output parsing" strategy,
// and the live CLI client (client.go) is the first consumer of a Profile.
package scheduler

import (
	"fmt"
	"strings"
)

// Canonical job states. Scheduler-specific tokens are mapped onto these via a
// Profile's StateMap so callers reason about one vocabulary.
const (
	StatePending   = "PENDING"
	StateRunning   = "RUNNING"
	StateCompleted = "COMPLETED"
	StateFailed    = "FAILED"
	StateCancelled = "CANCELLED"
	StateUnknown   = "UNKNOWN"
)

// Profile describes how to talk to one workload manager.
type Profile struct {
	// Name is the canonical scheduler identifier (e.g. "SLURM", "SGE", "TORQUE").
	Name string `mapstructure:"name" yaml:"name"`

	// SubmitViaStdin pipes the job script to the submit command via stdin
	// (sbatch/qsub all accept this) rather than passing a script file argument.
	SubmitViaStdin bool `mapstructure:"submit_via_stdin" yaml:"submit_via_stdin"`

	// ScriptHeader is prepended to every job script (e.g. PBS `#PBS` directives).
	ScriptHeader string `mapstructure:"script_header" yaml:"script_header"`

	// JobNameTemplate builds the job name from `{model}` and `{project}`
	// placeholders. Defaults to "{model}" when empty.
	JobNameTemplate string `mapstructure:"job_name_template" yaml:"job_name_template"`

	// Submit / Status / Cancel describe the three commands.
	Submit CommandSpec `mapstructure:"submit" yaml:"submit"`
	Status CommandSpec `mapstructure:"status" yaml:"status"`
	Cancel CommandSpec `mapstructure:"cancel" yaml:"cancel"`

	// SubmitIDPattern extracts the job ID from submit output via a named capture
	// group `id`.
	SubmitIDPattern string `mapstructure:"submit_id_pattern" yaml:"submit_id_pattern"`

	// StatusPattern parses status output via named capture groups `state` (and
	// optionally `id` to select the right row from a multi-job listing).
	StatusPattern string `mapstructure:"status_pattern" yaml:"status_pattern"`

	// StateMap maps a raw scheduler state token onto a canonical State* value.
	StateMap map[string]string `mapstructure:"state_map" yaml:"state_map"`
}

// CommandSpec is one scheduler command: the binary, any static arguments
// (with `{job}` substituted at render time), and conditional value-bearing
// flags that are only emitted when their source field is non-empty.
type CommandSpec struct {
	Command string     `mapstructure:"command" yaml:"command"`
	Args    []string   `mapstructure:"args" yaml:"args"`
	Flags   []FlagRule `mapstructure:"flags" yaml:"flags"`
}

// FlagRule emits Args (with `{}` replaced by the field value) only when the
// named JobSpec Field is non-empty. This mirrors how schedulers take optional
// flags (e.g. `--partition=X` only when a partition is set).
type FlagRule struct {
	Field string   `mapstructure:"field" yaml:"field"`
	Args  []string `mapstructure:"args" yaml:"args"`
}

// JobSpec carries the per-submission values a profile's flags draw from.
type JobSpec struct {
	JobName   string
	Partition string
	CPUs      int
	Memory    string
	TimeLimit string
	WorkDir   string
	Output    string
	Error     string
}

// builtins holds the default profiles, keyed by canonical (upper-case) name.
var builtins = map[string]Profile{
	"SLURM": {
		Name:           "SLURM",
		SubmitViaStdin: true,
		Submit: CommandSpec{
			Command: "sbatch",
			Flags: []FlagRule{
				{Field: "jobname", Args: []string{"--job-name={}"}},
				{Field: "partition", Args: []string{"--partition={}"}},
				{Field: "cpus", Args: []string{"--cpus-per-task={}"}},
				{Field: "memory", Args: []string{"--mem={}"}},
				{Field: "time", Args: []string{"--time={}"}},
				{Field: "workdir", Args: []string{"--chdir={}"}},
				{Field: "output", Args: []string{"--output={}"}},
				{Field: "error", Args: []string{"--error={}"}},
			},
		},
		SubmitIDPattern: `Submitted batch job (?P<id>\d+)`,
		Status: CommandSpec{
			Command: "squeue",
			Args:    []string{"--job={job}", "--format=%i,%T", "--noheader"},
		},
		StatusPattern: `(?P<id>\d+),(?P<state>\w+)`,
		StateMap: map[string]string{
			"PENDING":       StatePending,
			"CONFIGURING":   StatePending,
			"RUNNING":       StateRunning,
			"COMPLETING":    StateRunning,
			"COMPLETED":     StateCompleted,
			"FAILED":        StateFailed,
			"TIMEOUT":       StateFailed,
			"NODE_FAIL":     StateFailed,
			"OUT_OF_MEMORY": StateFailed,
			"CANCELLED":     StateCancelled,
		},
		Cancel: CommandSpec{Command: "scancel", Args: []string{"{job}"}},
	},
	"SGE": {
		Name:           "SGE",
		SubmitViaStdin: true,
		Submit: CommandSpec{
			Command: "qsub",
			Args:    []string{"-cwd"},
			Flags: []FlagRule{
				{Field: "jobname", Args: []string{"-N", "{}"}},
				{Field: "partition", Args: []string{"-q", "{}"}},
				{Field: "cpus", Args: []string{"-pe", "smp", "{}"}},
				{Field: "memory", Args: []string{"-l", "h_vmem={}"}},
				{Field: "time", Args: []string{"-l", "h_rt={}"}},
				{Field: "output", Args: []string{"-o", "{}"}},
				{Field: "error", Args: []string{"-e", "{}"}},
			},
		},
		SubmitIDPattern: `Your job (?P<id>\d+)`,
		Status:          CommandSpec{Command: "qstat"},
		StatusPattern:   `^\s*(?P<id>\d+)\s+\S+\s+\S+\s+\S+\s+(?P<state>\S+)`,
		StateMap: map[string]string{
			"r":    StateRunning,
			"t":    StateRunning,
			"qw":   StatePending,
			"hqw":  StatePending,
			"hRwq": StatePending,
			"Eqw":  StateFailed,
			"dr":   StateCancelled,
			"dt":   StateCancelled,
		},
		Cancel: CommandSpec{Command: "qdel", Args: []string{"{job}"}},
	},
	"TORQUE": {
		Name:           "TORQUE",
		SubmitViaStdin: true,
		Submit: CommandSpec{
			Command: "qsub",
			Flags: []FlagRule{
				{Field: "jobname", Args: []string{"-N", "{}"}},
				{Field: "partition", Args: []string{"-q", "{}"}},
				{Field: "cpus", Args: []string{"-l", "nodes=1:ppn={}"}},
				{Field: "memory", Args: []string{"-l", "mem={}"}},
				{Field: "time", Args: []string{"-l", "walltime={}"}},
				{Field: "output", Args: []string{"-o", "{}"}},
				{Field: "error", Args: []string{"-e", "{}"}},
			},
		},
		SubmitIDPattern: `(?P<id>\d+)`,
		Status:          CommandSpec{Command: "qstat", Args: []string{"-f", "{job}"}},
		StatusPattern:   `job_state\s*=\s*(?P<state>\w)`,
		StateMap: map[string]string{
			"R": StateRunning,
			"E": StateRunning,
			"S": StateRunning,
			"Q": StatePending,
			"H": StatePending,
			"W": StatePending,
			"T": StatePending,
			"C": StateCompleted,
			"F": StateCompleted,
		},
		Cancel: CommandSpec{Command: "qdel", Args: []string{"{job}"}},
	},
}

// schedulerAliases maps alternate scheduler names onto a built-in.
var schedulerAliases = map[string]string{
	"PBS": "TORQUE",
}

// BuiltinProfiles returns a copy of the built-in scheduler names.
func BuiltinProfiles() []string {
	names := make([]string, 0, len(builtins))
	for name := range builtins {
		names = append(names, name)
	}

	return names
}

// Resolve returns the Profile for a scheduler name. A user override (matched by
// Name, case-insensitively) wins over the built-in; aliases like PBS resolve to
// their base (TORQUE). It returns an error for unknown schedulers.
func Resolve(name string, overrides []Profile) (Profile, error) {
	key := strings.ToUpper(strings.TrimSpace(name))

	for _, o := range overrides {
		if strings.ToUpper(strings.TrimSpace(o.Name)) == key {
			return o, nil
		}
	}

	if alias, ok := schedulerAliases[key]; ok {
		key = alias
	}

	if p, ok := builtins[key]; ok {
		return p, nil
	}

	return Profile{}, fmt.Errorf("unknown scheduler %q", name)
}
