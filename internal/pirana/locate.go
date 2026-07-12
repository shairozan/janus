package pirana

import (
	"os"
	"path/filepath"
)

// piranaHomeEnv is Pirana's documented override for its home directory.
const piranaHomeEnv = "PIRANA_HOME"

// configFileNames are the global settings files Pirana may write, in order of
// preference: the modern SQLite database first, then the legacy INI.
var configFileNames = []string{"settings.db", "pirana.ini"}

// locateConfig searches the known Pirana home directories and returns the path
// to the first global settings file found. It returns ErrNotFound if none of
// the candidate locations contain a recognizable settings file.
func locateConfig() (string, error) {
	for _, home := range homeCandidates() {
		if home == "" {
			continue
		}

		for _, name := range configFileNames {
			candidate := filepath.Join(home, name)
			if fileExists(candidate) {
				return candidate, nil
			}
		}
	}

	return "", ErrNotFound
}

// homeCandidates returns the Pirana home directories to probe, most specific
// first. PIRANA_HOME (if set) always wins, followed by the cross-platform
// dot-directory and the platform-specific Windows locations.
func homeCandidates() []string {
	var dirs []string

	if env := os.Getenv(piranaHomeEnv); env != "" {
		dirs = append(dirs, env)
	}

	if home, err := os.UserHomeDir(); err == nil {
		// Linux/Mac default and the historic Windows location.
		dirs = append(dirs, filepath.Join(home, ".pirana"))
		dirs = append(dirs, filepath.Join(home, "pirana"))
	}

	// Windows modern (%LOCALAPPDATA%\.pirana) and legacy (%APPDATA%\Pirana).
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		dirs = append(dirs, filepath.Join(local, ".pirana"))
	}

	if appdata := os.Getenv("APPDATA"); appdata != "" {
		dirs = append(dirs, filepath.Join(appdata, "Pirana"))
	}

	return dirs
}

// fileExists reports whether path exists and is a regular file (not a directory).
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
