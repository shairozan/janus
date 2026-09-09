package execution

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shairozan/janus/internal/config"
)

// applyPreHooks runs the configured pre-run integration hooks.
func applyPreHooks(ctx context.Context, cfg *config.Config, modelPath string) {
	runIntegrationHooks(ctx, cfg, modelPath, "pre")
}

// applyPostHooks runs the configured post-run integration hooks (e.g. R
// diagnostics after a fit).
func applyPostHooks(ctx context.Context, cfg *config.Config, modelPath string) {
	runIntegrationHooks(ctx, cfg, modelPath, "post")
}

// runIntegrationHooks runs every configured hook matching `when` (best-effort:
// a failing diagnostic is logged but never affects the run).
func runIntegrationHooks(ctx context.Context, cfg *config.Config, modelPath, when string) {
	if cfg == nil {
		return
	}

	for _, h := range selectHooks(cfg.Integrations.Hooks, when) {
		if err := runIntegrationScript(ctx, cfg.Integrations.RPath, h.Script, modelPath); err != nil {
			log.Printf("Warning: integration hook %q (%s) failed: %v", h.Name, when, err)
		}
	}
}

// selectHooks returns the hooks whose When matches (case-insensitively) and that
// have a non-empty script.
func selectHooks(hooks []config.IntegrationHook, when string) []config.IntegrationHook {
	var out []config.IntegrationHook

	for _, h := range hooks {
		if strings.EqualFold(strings.TrimSpace(h.When), when) && strings.TrimSpace(h.Script) != "" {
			out = append(out, h)
		}
	}

	return out
}

// runIntegrationScript runs an R hook script (via Rscript) in the model's
// directory, passing the model path as an argument.
func runIntegrationScript(ctx context.Context, rPath, script, modelPath string) error {
	cmd := exec.CommandContext(ctx, rscriptExecutable(rPath), script, modelPath) //nolint:gosec // paths come from validated config
	cmd.Dir = filepath.Dir(modelPath)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s: %w", script, strings.TrimSpace(string(out)), err)
	}

	return nil
}

// rscriptExecutable resolves the Rscript command from the configured R path,
// which may be the Rscript binary itself or an R bin directory. Empty falls
// back to "Rscript" on PATH.
func rscriptExecutable(rPath string) string {
	if rPath == "" {
		return "Rscript"
	}

	if strings.Contains(strings.ToLower(filepath.Base(rPath)), "rscript") {
		return rPath
	}

	return filepath.Join(rPath, "Rscript")
}
