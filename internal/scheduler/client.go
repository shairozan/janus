package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// defaultTimeout bounds each scheduler command invocation.
const defaultTimeout = 30 * time.Second

// CLIClient runs a Profile's commands via the local shell — the generic,
// profile-driven replacement for the SLURM-only CLI client. It is the first
// consumer of a Profile; the same profiles drive command-generation tests.
type CLIClient struct {
	profile Profile
	timeout time.Duration
}

// NewCLIClient creates a CLI client for the given profile. A non-positive
// timeout falls back to defaultTimeout.
func NewCLIClient(profile Profile, timeout time.Duration) *CLIClient {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &CLIClient{profile: profile, timeout: timeout}
}

// Profile returns the profile backing this client.
func (c *CLIClient) Profile() Profile {
	return c.profile
}

// Submit submits a job script and returns the parsed job ID. The script is
// piped via stdin (with the profile's ScriptHeader prepended) and the command
// runs in spec.WorkDir when set.
func (c *CLIClient) Submit(ctx context.Context, script string, spec JobSpec) (string, error) {
	argv := RenderSubmit(c.profile, spec)
	if len(argv) == 0 {
		return "", fmt.Errorf("scheduler %q has no submit command", c.profile.Name)
	}

	body := script
	if c.profile.ScriptHeader != "" {
		body = c.profile.ScriptHeader + "\n" + script
	}

	out, err := c.run(ctx, argv, spec.WorkDir, body)
	if err != nil {
		return "", fmt.Errorf("submit failed: %w", err)
	}

	return ParseJobID(c.profile, out)
}

// Status returns the canonical state of jobID.
func (c *CLIClient) Status(ctx context.Context, jobID string) (string, error) {
	argv := RenderStatus(c.profile, jobID)
	if len(argv) == 0 {
		return StateUnknown, fmt.Errorf("scheduler %q has no status command", c.profile.Name)
	}

	out, err := c.run(ctx, argv, "", "")
	if err != nil {
		return StateUnknown, fmt.Errorf("status query failed: %w", err)
	}

	return ParseState(c.profile, out, jobID)
}

// Cancel cancels jobID.
func (c *CLIClient) Cancel(ctx context.Context, jobID string) error {
	argv := RenderCancel(c.profile, jobID)
	if len(argv) == 0 {
		return fmt.Errorf("scheduler %q has no cancel command", c.profile.Name)
	}

	if _, err := c.run(ctx, argv, "", ""); err != nil {
		return fmt.Errorf("cancel failed: %w", err)
	}

	return nil
}

// run executes argv with the client's timeout, optional working directory, and
// optional stdin, returning combined stdout. Stderr is surfaced on failure.
func (c *CLIClient) run(ctx context.Context, argv []string, workDir, stdin string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // argv comes from a trusted scheduler profile
	if workDir != "" {
		cmd.Dir = workDir
	}

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%s: %s", argv[0], strings.TrimSpace(string(exitErr.Stderr)))
		}

		return "", fmt.Errorf("failed to execute %s: %w", argv[0], err)
	}

	return string(out), nil
}
