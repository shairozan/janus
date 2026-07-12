package runlog

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// EmbedOutputFiles captures the model's output files of interest and embeds them
// in the run record, keyed by their path relative to the model directory (e.g.
// "run1.lst", "sdtab1", "psn_janus/m1/run1.lst"). The set is the model control
// file (always, so summary extraction keeps parameter labels) plus every file
// matching the resolved retain globs. Files under MaxEmbeddedFileSize are
// compressed and embedded as base64; larger files and directories are skipped
// (future: external storage).
//
// retain is the resolved retain glob set (see config.ResolveRetain). Globs are
// matched with doublestar (the same matcher Hermes uses), so recursive patterns
// like "psn_janus/**" match the whole PsN run tree and land in the run log. Keys
// are relative paths (forward-slashed) so files with the same base name in
// different subdirectories do not collide.
func EmbedOutputFiles(record *RunRecord, modelPath string, retain []string) error {
	if record.EmbeddedFiles == nil {
		record.EmbeddedFiles = make(map[string]string)
	}

	modelDir := filepath.Dir(modelPath)

	// Build the set of paths to embed. The control file is always included so
	// ExtractSummary can read parameter labels regardless of the retain set.
	paths := map[string]struct{}{}
	if _, err := os.Stat(modelPath); err == nil {
		paths[modelPath] = struct{}{}
	}

	for _, pattern := range retain {
		// doublestar matches "**" (Hermes does the same server-side), so recursive
		// PsN patterns resolve here too.
		matches, err := doublestar.FilepathGlob(filepath.Join(modelDir, pattern))
		if err != nil {
			// doublestar only errors on a malformed pattern; surface it rather than
			// silently dropping the file.
			return fmt.Errorf("invalid retain glob %q: %w", pattern, err)
		}

		for _, m := range matches {
			paths[m] = struct{}{}
		}
	}

	for path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				// A match that vanished, or a control file that was never written.
				continue
			}

			return fmt.Errorf("failed to stat file %s: %w", path, err)
		}

		// A glob can match a directory; embedding is file-only. Skip oversized
		// files (future: external storage).
		if info.IsDir() || !ShouldEmbed(info.Size()) {
			continue
		}

		encoded, err := CompressFile(path)
		if err != nil {
			return fmt.Errorf("failed to compress file %s: %w", path, err)
		}

		record.EmbeddedFiles[embedKey(modelDir, path)] = encoded
	}

	return nil
}

// embedKey returns the map key for an embedded file: its path relative to the
// model directory, forward-slashed for portability. Files directly in the model
// directory key by their base name; nested files (e.g. a PsN run tree) keep their
// subpath so identical base names don't collide. Falls back to the base name when
// a relative path can't be computed.
func embedKey(modelDir, path string) string {
	rel, err := filepath.Rel(modelDir, path)
	if err != nil {
		return filepath.Base(path)
	}

	return filepath.ToSlash(rel)
}

// ExtractEmbeddedFile decompresses and returns the content of the embedded file
// with the given key — its base filename, e.g. "run1.lst" or "sdtab1".
func ExtractEmbeddedFile(record *RunRecord, name string) ([]byte, error) {
	encoded, exists := record.EmbeddedFiles[name]
	if !exists {
		return nil, fmt.Errorf("embedded file %q not found", name)
	}

	return DecodeAndDecompress(encoded)
}

// GetEmbeddedFileNames returns the base filenames of the embedded files.
func GetEmbeddedFileNames(record *RunRecord) []string {
	names := make([]string, 0, len(record.EmbeddedFiles))
	for name := range record.EmbeddedFiles {
		names = append(names, name)
	}

	return names
}

// EmbeddedNameForExt returns the key of the first embedded file whose extension
// matches ext (with or without a leading dot), or "" when none is present. It
// lets extension-oriented consumers (summary extraction) locate a file now that
// embedded files are keyed by filename rather than by extension.
func EmbeddedNameForExt(record *RunRecord, ext string) string {
	want := "." + strings.TrimPrefix(ext, ".")
	for name := range record.EmbeddedFiles {
		if strings.EqualFold(filepath.Ext(name), want) {
			return name
		}
	}

	return ""
}

// CalculateChecksum computes SHA256 checksum of data.
func CalculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)

	return fmt.Sprintf("%x", hash)
}

// CreateFileRef creates a FileRef for external storage.
// This is a placeholder for future pluggable storage backends.
func CreateFileRef(filepath string, backend string) (FileRef, error) {
	info, err := os.Stat(filepath)
	if err != nil {
		return FileRef{}, fmt.Errorf("failed to stat file: %w", err)
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return FileRef{}, fmt.Errorf("failed to read file: %w", err)
	}

	return FileRef{
		Path:        filepath,
		Size:        info.Size(),
		Checksum:    CalculateChecksum(data),
		ContentType: "text/plain", // TODO: Detect based on extension
		Backend:     backend,
	}, nil
}
