package gui

import (
	"errors"
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"

	"github.com/shairozan/janus/internal/appsetup"
	"github.com/shairozan/janus/internal/mcpservice"
	"github.com/shairozan/janus/internal/runlog"
)

// This file is the GUI's thin seam onto the shared, fyne-free mcpservice. The
// service owns the bridge implementation and the single headless execution path;
// the GUI contributes only two things the daemon lacks: a "default to the
// currently-loaded model" store resolver, and a post-run hook that refreshes the
// run-history view.

// buildMCPService constructs the mcpservice.Service backing the MCP server, wired
// to the GUI's resolver, run-complete hook, and error channel. The ephemeral
// resolver (for models other than the loaded one) is built once and reused.
func (a *App) buildMCPService() (*mcpservice.Service, error) {
	if a.ephemeralResolver == nil {
		a.ephemeralResolver = mcpservice.NewEphemeralResolver(a.signer, a.signerEmail())
	}

	return mcpservice.New(mcpservice.Options{
		Config:     a.config,
		Resolve:    a.resolveStore,
		AppCtx:     a.errorCtx,
		OnComplete: a.onRunComplete,
		ErrSink:    a.sendError,
	})
}

// signerEmail returns the identity bound to the run-log signing key, or "".
//
// The identity comes from config rather than from the OS user: it labels the
// key, and the trust keyring matches it to a public key. Deriving it per-run
// would let the identity recorded on a record drift from the key that actually
// signed it.
func (a *App) signerEmail() string {
	return appsetup.SignerIdentity(a.config)
}

// resolveStore is the GUI's StoreResolver: an empty path (or a path matching the
// loaded model) resolves to the live run-log store; any other path is delegated
// to the shared ephemeral resolver. Reusing the live store for the loaded model
// keeps the GUI and the bridge on the same in-process store instance.
func (a *App) resolveStore(modelPath string) (*runlog.RunLogStore, string, error) {
	a.modelMu.Lock()
	current := a.currentFilePath
	currentStore := a.runLogStore
	a.modelMu.Unlock()

	target := modelPath
	if target == "" {
		target = current
	}

	if target == "" {
		return nil, "", errors.New("no model loaded and no model_path provided")
	}

	abs, err := filepath.Abs(target)
	if err != nil {
		return nil, "", fmt.Errorf("invalid model path %q: %w", target, err)
	}

	if currentStore != nil && current != "" {
		if currentAbs, absErr := filepath.Abs(current); absErr == nil && currentAbs == abs {
			return currentStore, abs, nil
		}
	}

	return a.ephemeralResolver.Resolve(target)
}

// onRunComplete refreshes the run-history UI only when a headless run targeted
// the currently-loaded model, so daemon-style runs against the loaded model show
// up immediately while runs against other models never touch the UI.
func (a *App) onRunComplete(store *runlog.RunLogStore) {
	a.modelMu.Lock()
	live := a.runLogStore
	a.modelMu.Unlock()

	if store != live {
		return
	}

	a.refreshCachedRuns()

	if a.runHistoryTable != nil {
		fyne.Do(func() {
			a.runHistoryTable.Refresh()
		})
	}
}
