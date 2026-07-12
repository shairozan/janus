package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// profilesSubdir is the directory (next to the active config file) where named
// configuration profile snapshots are stored.
const profilesSubdir = "profiles"

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() string {
	return getDefaultConfigPath()
}

// profilesDirFor returns the profiles directory for a given config file path.
func profilesDirFor(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), profilesSubdir)
}

// profilePath returns the file path for a named profile. filepath.Base strips any
// directory components from name as defense-in-depth against path traversal (the
// callers also reject such names via validateProfileName).
func profilePath(configPath, name string) string {
	return filepath.Join(profilesDirFor(configPath), filepath.Base(name)+".yml")
}

// validateProfileName rejects names that are not a single path element, so a
// crafted name (path separators or "..") cannot escape the profiles directory.
func validateProfileName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("profile name is required")
	}

	if trimmed == ".." || strings.ContainsAny(trimmed, `/\`) {
		return fmt.Errorf("invalid profile name %q: must not contain path separators", name)
	}

	return nil
}

// ListProfiles returns the names of saved profiles (without extension), sorted.
// A missing profiles directory yields an empty list, not an error.
func ListProfiles(configPath string) ([]string, error) {
	entries, err := os.ReadDir(profilesDirFor(configPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var names []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		if ext := filepath.Ext(e.Name()); ext == ".yml" || ext == ".yaml" {
			names = append(names, strings.TrimSuffix(e.Name(), ext))
		}
	}

	sort.Strings(names)

	return names, nil
}

// SaveProfile snapshots the active config file as a named profile.
func SaveProfile(configPath, name string) error {
	if err := validateProfileName(name); err != nil {
		return err
	}

	if err := os.MkdirAll(profilesDirFor(configPath), 0o755); err != nil {
		return fmt.Errorf("create profiles dir: %w", err)
	}

	return copyFileContents(configPath, profilePath(configPath, name))
}

// SwitchProfile makes a named profile the active config (overwriting the active
// config file with the profile's contents).
func SwitchProfile(configPath, name string) error {
	if err := validateProfileName(name); err != nil {
		return err
	}

	src := profilePath(configPath, name)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("profile %q not found", name)
	}

	return copyFileContents(src, configPath)
}

// DeleteProfile removes a named profile.
func DeleteProfile(configPath, name string) error {
	if err := validateProfileName(name); err != nil {
		return err
	}

	return os.Remove(profilePath(configPath, name))
}

// copyFileContents copies src to dst (file-mode 0600 for config files).
func copyFileContents(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}

	return nil
}
