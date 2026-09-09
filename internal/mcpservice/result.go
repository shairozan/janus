package mcpservice

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/shairozan/janus/internal/execution"
	"github.com/shairozan/janus/internal/extraction"
	"github.com/shairozan/janus/internal/mcp"
	"github.com/shairozan/janus/internal/runlog"
)

// commandBuilder is implemented by the NONMEM/PSN/BBI executors. It lets the
// headless path render the exact command for a dry run without running it.
type commandBuilder interface {
	BuildCommand(modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error)
}

// ApplyRunResult writes an execution result onto the run record identified by
// runID in store: it sets the exit code, stdout/stderr, container provenance and
// status, and for successful runs embeds output files and extracts the model
// summary. It then re-signs and persists the record. It performs no UI work, so
// it is shared by the GUI's updateRunRecord and the headless execution path.
//
// retain is the resolved retain glob set (see config.ResolveRetain) — the model's
// output files of interest — used to select which files are embedded in the run
// log. The caller resolves it because config resolution (per-model → global →
// category default) lives above this layer. ctx is threaded to the summary
// extraction so the parse honors the caller's cancellation/timeout.
func ApplyRunResult(ctx context.Context, store *runlog.RunLogStore, runID string, exitCode int, stdout, stderr string, container *runlog.ContainerProvenance, retain []string) error {
	record, err := store.GetRun(runID)
	if err != nil {
		return fmt.Errorf("failed to get run record %s: %w", runID, err)
	}

	record.ExitCode = exitCode

	if err := record.SetStdout(stdout); err != nil {
		record.Stdout = stdout // Fallback to uncompressed
	}

	if err := record.SetStderr(stderr); err != nil {
		record.Stderr = stderr // Fallback to uncompressed
	}

	if container != nil {
		record.Container = container
	}

	if exitCode == 0 {
		record.Status = "completed"

		if err := runlog.EmbedOutputFiles(record, record.ModelFile, retain); err != nil {
			log.Printf("Warning: failed to embed output files: %v", err)
		}

		if summary, err := extraction.ExtractSummary(ctx, record); err != nil {
			log.Printf("Warning: failed to extract model summary: %v", err)
		} else if summary != nil {
			record.Summary = summary
		}

		runDir := filepath.Dir(record.ModelFile)
		if tableDiag, err := extraction.ExtractTableDiagnostics(record, runDir); err != nil {
			log.Printf("Warning: failed to extract table diagnostics: %v", err)
		} else if tableDiag != nil && record.Summary != nil {
			record.Summary.TableDiagnostics = tableDiag
		}
	} else {
		record.Status = "failed"
	}

	if err := store.UpdateRun(record); err != nil {
		return fmt.Errorf("failed to update run record: %w", err)
	}

	return nil
}

// WritePnmFile writes a NONMEM parallel (.pnm) configuration alongside modelPath.
// It is path-based so both the GUI and the headless path can call it.
func WritePnmFile(modelPath string, cores int) error {
	modelDir := filepath.Dir(modelPath)
	modelName := strings.TrimSuffix(filepath.Base(modelPath), filepath.Ext(modelPath))
	pnmPath := filepath.Join(modelDir, modelName+".pnm")

	pnmContent := fmt.Sprintf(`$GENERAL
NODES=%d PARSE_TYPE=2 TIMEOUTI=100 TIMEOUT=2400 PARAPRINT=0 TRANSFER_TYPE=1
`, cores)

	if err := os.WriteFile(pnmPath, []byte(pnmContent), 0600); err != nil {
		return fmt.Errorf("failed to write .pnm file: %w", err)
	}

	return nil
}

// headlessCommand renders the command that would run, using the executor's
// BuildCommand when available (NONMEM/PSN/BBI) and falling back to a descriptive
// string for executors that do not build a shell command (e.g. Hermes).
func headlessCommand(executor execution.Executor, modelPath string, req mcp.ExecuteRequest, additionalOptions []string) string {
	if builder, ok := executor.(commandBuilder); ok {
		binary, args, err := builder.BuildCommand(modelPath, req.IsParallel, req.Cores, req.IsGrid, additionalOptions)
		if err == nil {
			return strings.TrimSpace(binary + " " + strings.Join(args, " "))
		}
	}

	return fmt.Sprintf("execute %s", filepath.Base(modelPath))
}
