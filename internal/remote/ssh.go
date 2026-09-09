package remote

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/scheduler"
)

// sshTimeout bounds connection establishment and each remote command.
const sshTimeout = 30 * time.Second

// defaultSSHPort is used when no port is configured.
const defaultSSHPort = 22

// Runner executes arbitrary commands on a remote host over SSH (key-based auth).
// It is the generic transport shared by the scheduler client and PsN.
type Runner struct {
	cfg     config.RemoteConfig
	auth    []ssh.AuthMethod
	hostKey ssh.HostKeyCallback
}

// NewRunner builds a remote runner, loading the private key, and returns an
// error if the host/key are missing or the key cannot be parsed.
func NewRunner(cfg config.RemoteConfig) (*Runner, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("remote host is required")
	}

	if cfg.KeyPath == "" {
		return nil, fmt.Errorf("remote key_path is required")
	}

	keyBytes, err := os.ReadFile(cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("read SSH key %s: %w", cfg.KeyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse SSH key: %w", err)
	}

	return &Runner{
		cfg:     cfg,
		auth:    []ssh.AuthMethod{ssh.PublicKeys(signer)},
		hostKey: defaultHostKeyCallback(),
	}, nil
}

// Run executes argv on the remote host (optionally cd'd into workDir, with
// optional stdin) and returns combined stdout and the command's exit code. The
// returned error is non-nil only for transport/connection failures — a non-zero
// command exit is reported via exitCode, not err.
func (r *Runner) Run(ctx context.Context, workDir string, argv []string, stdin string) (string, int, error) {
	addr := net.JoinHostPort(r.cfg.Host, strconv.Itoa(r.port()))

	clientCfg := &ssh.ClientConfig{
		User:            r.cfg.User,
		Auth:            r.auth,
		HostKeyCallback: r.hostKey,
		Timeout:         sshTimeout,
	}

	dialer := net.Dialer{Timeout: sshTimeout}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", 0, fmt.Errorf("dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientCfg)
	if err != nil {
		conn.Close()

		return "", 0, fmt.Errorf("ssh handshake with %s: %w", addr, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", 0, fmt.Errorf("open ssh session: %w", err)
	}
	defer session.Close()

	if stdin != "" {
		session.Stdin = strings.NewReader(stdin)
	}

	out, err := session.Output(buildRemoteCommand(workDir, argv))
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			// The command ran but exited non-zero — report the code, not an error.
			return string(out), exitErr.ExitStatus(), nil
		}

		return "", 0, fmt.Errorf("remote command failed: %w", err)
	}

	return string(out), 0, nil
}

// port returns the configured SSH port or the default.
func (r *Runner) port() int {
	if r.cfg.Port > 0 {
		return r.cfg.Port
	}

	return defaultSSHPort
}

// SSHClient runs a scheduler Profile's commands on a remote host over SSH. It
// implements the same Submit/Status/Cancel contract as scheduler.CLIClient, so
// the grid executor can use either interchangeably.
type SSHClient struct {
	profile scheduler.Profile
	runner  *Runner
}

// NewSSHClient builds a scheduler client over a remote SSH runner.
func NewSSHClient(profile scheduler.Profile, cfg config.RemoteConfig) (*SSHClient, error) {
	runner, err := NewRunner(cfg)
	if err != nil {
		return nil, err
	}

	return &SSHClient{profile: profile, runner: runner}, nil
}

// run executes argv and returns stdout, treating a non-zero exit as an error
// (scheduler poll/cancel commands are expected to succeed).
func (c *SSHClient) run(ctx context.Context, workDir string, argv []string, stdin string) (string, error) {
	out, code, err := c.runner.Run(ctx, workDir, argv, stdin)
	if err != nil {
		return "", err
	}

	if code != 0 {
		return "", fmt.Errorf("remote command exited %d: %s", code, strings.TrimSpace(out))
	}

	return out, nil
}

// Submit submits a job script over SSH and returns the parsed job ID. The script
// is piped via stdin; spec.WorkDir is the remote working directory.
func (c *SSHClient) Submit(ctx context.Context, script string, spec scheduler.JobSpec) (string, error) {
	body := script
	if c.profile.ScriptHeader != "" {
		body = c.profile.ScriptHeader + "\n" + script
	}

	out, err := c.run(ctx, spec.WorkDir, scheduler.RenderSubmit(c.profile, spec), body)
	if err != nil {
		return "", fmt.Errorf("remote submit failed: %w", err)
	}

	return scheduler.ParseJobID(c.profile, out)
}

// Status returns the canonical state of jobID.
func (c *SSHClient) Status(ctx context.Context, jobID string) (string, error) {
	out, err := c.run(ctx, "", scheduler.RenderStatus(c.profile, jobID), "")
	if err != nil {
		return scheduler.StateUnknown, fmt.Errorf("remote status query failed: %w", err)
	}

	return scheduler.ParseState(c.profile, out, jobID)
}

// Cancel cancels jobID.
func (c *SSHClient) Cancel(ctx context.Context, jobID string) error {
	if _, err := c.run(ctx, "", scheduler.RenderCancel(c.profile, jobID), ""); err != nil {
		return fmt.Errorf("remote cancel failed: %w", err)
	}

	return nil
}

// defaultHostKeyCallback verifies host keys against the user's known_hosts file
// when available; otherwise it falls back to accepting the host key (documented
// for first-connection usability).
func defaultHostKeyCallback() ssh.HostKeyCallback {
	if home, err := os.UserHomeDir(); err == nil {
		khPath := filepath.Join(home, ".ssh", "known_hosts")
		if cb, err := knownhosts.New(khPath); err == nil {
			return cb
		}
	}

	return ssh.InsecureIgnoreHostKey() //nolint:gosec // no known_hosts available; accept on first use
}
