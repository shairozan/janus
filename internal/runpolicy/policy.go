// Package runpolicy applies a run's output-directory policy: overwrite
// protection (sequential numbered dirs), optional backup of model + results,
// and post-run cleanup of intermediate files. It is shared lower-layer code so
// both the GUI-driven executors and the headless executor binary apply the same
// policy. With a zero-value Policy every operation is a no-op, so existing
// behavior is unchanged unless a user opts in.
package runpolicy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Overwrite policy values.
const (
	// OverwriteInPlace lets a re-run overwrite prior outputs (default).
	OverwriteInPlace = "overwrite"

	// OverwriteSequential archives prior result files into a numbered
	// modelfit_dirN before a re-run, so nothing is lost.
	OverwriteSequential = "sequential"
)

// maxSequential bounds the search for the next numbered directory.
const maxSequential = 10000

// Policy describes how a run's outputs are managed.
type Policy struct {
	// Overwrite is OverwriteInPlace (default) or OverwriteSequential.
	Overwrite string

	// AutoBackup copies the model and its results into a backup/ subfolder
	// after a run completes.
	AutoBackup bool

	// CleanupGlobs are shell globs (relative to the model directory) whose
	// matching files are deleted after a run completes.
	CleanupGlobs []string
}

// sequential reports whether the sequential overwrite policy is active.
func (p Policy) sequential() bool {
	return strings.EqualFold(p.Overwrite, OverwriteSequential)
}

// BeforeRun prepares the model directory for a run. When the sequential policy
// is active and prior result files exist, they are moved into the next
// modelfit_dirN so the new run cannot overwrite them. It returns the archive
// directory (empty when nothing was archived).
func (p Policy) BeforeRun(modelPath string) (string, error) {
	if !p.sequential() {
		return "", nil
	}

	modelDir := filepath.Dir(modelPath)
	modelName := filepath.Base(modelPath)

	priors, err := stemFiles(modelDir, stemOf(modelName), modelName)
	if err != nil {
		return "", err
	}

	if len(priors) == 0 {
		return "", nil
	}

	archive, err := NextSequentialDir(modelDir)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(archive, 0o755); err != nil {
		return "", fmt.Errorf("create archive dir: %w", err)
	}

	for _, name := range priors {
		from := filepath.Join(modelDir, name)
		to := filepath.Join(archive, name)
		if err := os.Rename(from, to); err != nil {
			return "", fmt.Errorf("archive %s: %w", name, err)
		}
	}

	return archive, nil
}

// AfterRun runs post-run housekeeping: optional backup of the model and its
// results, then optional cleanup of intermediate files. It is a no-op when
// neither is configured.
func (p Policy) AfterRun(modelPath string) error {
	modelDir := filepath.Dir(modelPath)
	modelName := filepath.Base(modelPath)

	if p.AutoBackup {
		if err := p.backup(modelDir, modelName); err != nil {
			return err
		}
	}

	if len(p.CleanupGlobs) > 0 {
		if err := p.cleanup(modelDir); err != nil {
			return err
		}
	}

	return nil
}

// backup copies the model and every `stem.*` result file into a fresh numbered
// backup/<stem> directory.
func (p Policy) backup(modelDir, modelName string) error {
	files, err := stemFiles(modelDir, stemOf(modelName), "")
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	dest, err := nextBackupDir(modelDir, stemOf(modelName))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	for _, name := range files {
		if err := copyFile(filepath.Join(modelDir, name), filepath.Join(dest, name)); err != nil {
			return fmt.Errorf("backup %s: %w", name, err)
		}
	}

	return nil
}

// cleanup deletes files (not directories) matching the configured globs.
func (p Policy) cleanup(modelDir string) error {
	for _, glob := range p.CleanupGlobs {
		matches, err := filepath.Glob(filepath.Join(modelDir, glob))
		if err != nil {
			return fmt.Errorf("invalid cleanup glob %q: %w", glob, err)
		}

		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || info.IsDir() {
				continue
			}

			if err := os.Remove(m); err != nil {
				return fmt.Errorf("cleanup %s: %w", m, err)
			}
		}
	}

	return nil
}

// NextSequentialDir returns the next available modelfit_dirN path in modelDir.
func NextSequentialDir(modelDir string) (string, error) {
	for i := 1; i <= maxSequential; i++ {
		cand := filepath.Join(modelDir, fmt.Sprintf("modelfit_dir%d", i))

		_, err := os.Stat(cand)
		if os.IsNotExist(err) {
			return cand, nil
		}

		if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("exhausted modelfit_dir search in %s", modelDir)
}

// nextBackupDir returns a fresh backup/<stem>[-N] directory under modelDir.
func nextBackupDir(modelDir, stem string) (string, error) {
	base := filepath.Join(modelDir, "backup")

	for i := 0; i <= maxSequential; i++ {
		cand := filepath.Join(base, stem)
		if i > 0 {
			cand = filepath.Join(base, fmt.Sprintf("%s-%d", stem, i))
		}

		_, err := os.Stat(cand)
		if os.IsNotExist(err) {
			return cand, nil
		}

		if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("exhausted backup dir search in %s", base)
}

// stemFiles returns the names of files in dir whose name begins with `stem.`,
// excluding `exclude`. Subdirectories are skipped.
func stemFiles(dir, stem, exclude string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	prefix := stem + "."

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if name == exclude {
			continue
		}

		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}

	return names, nil
}

// stemOf returns a file name without its extension.
func stemOf(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// copyFile copies src to dst, preserving file mode.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()

		return err
	}

	return out.Close()
}
