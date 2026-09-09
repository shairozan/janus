// Package mcpservice is the fyne-free core that implements mcp.Bridge from
// Janus's run-log and execution layers. Both the GUI (via a thin wrapper that
// adds "default to the currently-loaded model") and the `janus mcp server`
// daemon construct a Service and hand it to an mcp.Server, so a single execution
// path backs both. It imports no internal/gui and no fyne.
package mcpservice

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/shairozan/janus/internal/runlog"
	"github.com/shairozan/janus/internal/signing"
)

// StoreResolver maps a model path to its run-log store. An empty path is the
// caller's "default" model (the GUI resolves it to the loaded model; the daemon
// has no default and returns an error). It returns the resolved absolute model
// path alongside the store.
type StoreResolver func(modelPath string) (store *runlog.RunLogStore, resolvedPath string, err error)

// EphemeralResolver builds and caches run-log stores by absolute model path,
// configuring each with the given signer. It is the resolver used directly by
// the daemon and composed by the GUI for models other than the loaded one.
type EphemeralResolver struct {
	signer      *signing.Signer
	signerEmail string

	mu     sync.Mutex
	stores map[string]*runlog.RunLogStore
}

// NewEphemeralResolver returns a resolver that builds per-path stores. signer may
// be nil (signing disabled).
func NewEphemeralResolver(signer *signing.Signer, signerEmail string) *EphemeralResolver {
	return &EphemeralResolver{
		signer:      signer,
		signerEmail: signerEmail,
		stores:      make(map[string]*runlog.RunLogStore),
	}
}

// Resolve implements the StoreResolver contract. An empty model path is an error
// here (the daemon has no current model); the GUI supplies its own default
// before delegating.
func (r *EphemeralResolver) Resolve(modelPath string) (*runlog.RunLogStore, string, error) {
	if modelPath == "" {
		return nil, "", errors.New("no model_path provided")
	}

	abs, err := filepath.Abs(modelPath)
	if err != nil {
		return nil, "", fmt.Errorf("invalid model path %q: %w", modelPath, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if store, ok := r.stores[abs]; ok {
		return store, abs, nil
	}

	dir := filepath.Dir(abs)
	name := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	store := runlog.NewRunLogStore(dir, name)

	if r.signer != nil {
		store.SetSigner(r.signer, r.signerEmail)
	}

	if err := store.Load(); err != nil {
		return nil, "", fmt.Errorf("failed to load run log for %q: %w", abs, err)
	}

	r.stores[abs] = store

	return store, abs, nil
}
