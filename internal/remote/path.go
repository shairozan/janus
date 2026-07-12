// Package remote provides SSH-based execution of scheduler commands on a remote
// cluster, with local↔remote path translation. Local model paths are rewritten
// to their remote equivalents before commands are sent over SSH; results are
// read back locally through the shared mount, so callers collect output exactly
// as they do for local runs.
//
// Path translation and remote-command assembly are pure and unit-tested (no live
// cluster required), per the IQ/OQ "validate generation + parsing" strategy.
package remote

import (
	"path/filepath"
	"strings"

	"github.com/pharmalytica/janus/internal/config"
)

// PathMapper translates paths between the local filesystem and a remote host
// using configured mount mappings.
type PathMapper struct {
	mounts []config.RemoteMount
}

// NewPathMapper builds a mapper from mount mappings.
func NewPathMapper(mounts []config.RemoteMount) *PathMapper {
	return &PathMapper{mounts: mounts}
}

// HasMounts reports whether any mount mappings are configured.
func (m *PathMapper) HasMounts() bool {
	return m != nil && len(m.mounts) > 0
}

// ToRemote rewrites a local path to its remote equivalent using the longest
// matching local mount. Remote paths use forward slashes. A path under no mount
// is returned unchanged.
func (m *PathMapper) ToRemote(localPath string) string {
	norm := toSlash(localPath)

	bestLen := -1
	result := localPath

	for _, mt := range m.mounts {
		local := strings.TrimRight(toSlash(mt.Local), "/")
		if !hasPathPrefix(norm, local) {
			continue
		}

		if len(local) > bestLen {
			bestLen = len(local)
			rest := norm[len(local):]
			result = strings.TrimRight(toSlash(mt.Remote), "/") + rest
		}
	}

	return result
}

// ToLocal rewrites a remote path to its local equivalent using the longest
// matching remote mount, returning an OS-native path. A path under no mount is
// returned unchanged.
func (m *PathMapper) ToLocal(remotePath string) string {
	norm := toSlash(remotePath)

	bestLen := -1
	result := remotePath

	for _, mt := range m.mounts {
		remote := strings.TrimRight(toSlash(mt.Remote), "/")
		if !hasPathPrefix(norm, remote) {
			continue
		}

		if len(remote) > bestLen {
			bestLen = len(remote)
			rest := norm[len(remote):]
			result = filepath.FromSlash(strings.TrimRight(toSlash(mt.Local), "/") + rest)
		}
	}

	return result
}

// toSlash normalizes separators to forward slashes for comparison.
func toSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// hasPathPrefix reports whether p lies under prefix at a path-component
// boundary. Matching is case-insensitive to tolerate Windows drive letters.
func hasPathPrefix(p, prefix string) bool {
	if prefix == "" {
		return false
	}

	lp := strings.ToLower(p)
	lpre := strings.ToLower(prefix)

	if lp == lpre {
		return true
	}

	return strings.HasPrefix(lp, lpre+"/")
}
