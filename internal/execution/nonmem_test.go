//go:build unit
// +build unit

package execution

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildNonmemBinaryPath(t *testing.T) {
	tests := []struct {
		name         string
		nonmemPath   string
		nonmemBinary string
		wantContains []string // Expected parts that should be in the result
	}{
		{
			name:         "Linux absolute path",
			nonmemPath:   "/opt/NONMEM/nm76/run",
			nonmemBinary: "nmfe76",
			wantContains: []string{"opt", "NONMEM", "nm76", "run", "nmfe76"},
		},
		{
			name:         "Linux relative path",
			nonmemPath:   "opt/NONMEM",
			nonmemBinary: "nmfe76",
			wantContains: []string{"opt", "NONMEM", "nmfe76"},
		},
		{
			name:         "Windows absolute path",
			nonmemPath:   "C:/NONMEM",
			nonmemBinary: "nmfe76",
			wantContains: []string{"NONMEM", "nmfe76"},
		},
		{
			name:         "Windows backslash path",
			nonmemPath:   "C:\\NONMEM",
			nonmemBinary: "nmfe76.bat",
			wantContains: []string{"NONMEM", "nmfe76.bat"},
		},
		{
			name:         "Empty paths",
			nonmemPath:   "",
			nonmemBinary: "nmfe76",
			wantContains: []string{"nmfe76"},
		},
		{
			name:         "Path with spaces",
			nonmemPath:   "/opt/My NONMEM/nm76",
			nonmemBinary: "nmfe76",
			wantContains: []string{"opt", "My NONMEM", "nm76", "nmfe76"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildNonmemBinaryPath(tt.nonmemPath, tt.nonmemBinary)

			// Check that no error occurred
			if err != nil {
				t.Errorf("buildNonmemBinaryPath() returned error: %v", err)
				return
			}

			// Check that result is not empty
			if result == "" {
				t.Errorf("buildNonmemBinaryPath() returned empty string")
				return
			}

			// Check that all expected parts are contained in the result
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("buildNonmemBinaryPath() = %q, want to contain %q", result, want)
				}
			}

			// Verify the result is a valid path format
			if !filepath.IsAbs(result) && tt.nonmemPath != "" && !strings.HasPrefix(tt.nonmemPath, ".") {
				// For non-empty, non-relative paths, we expect an absolute result
				// (unless it's a relative path like "opt/NONMEM")
				if filepath.IsAbs(tt.nonmemPath) {
					t.Errorf("buildNonmemBinaryPath() = %q, expected absolute path", result)
				}
			}

			// Verify path separators are appropriate for the current OS
			// Skip separator validation for cross-platform test cases (e.g., testing Windows paths on Linux)
			expectedSep := string(filepath.Separator)
			isWindowsPath := strings.Contains(tt.nonmemPath, ":\\") || strings.Contains(tt.nonmemPath, "C:/")
			isUnixPath := strings.HasPrefix(tt.nonmemPath, "/")

			// Only validate separators for native platform paths
			if runtime.GOOS == "windows" && (isWindowsPath || !isUnixPath) {
				// On Windows, both / and \ are valid, but filepath should normalize to \
				if !strings.Contains(result, expectedSep) && !strings.Contains(result, "/") {
					t.Errorf("buildNonmemBinaryPath() = %q, expected path separators", result)
				}
			} else if runtime.GOOS != "windows" && isUnixPath {
				// On Unix-like systems, only validate Unix paths
				if strings.Contains(result, "\\") {
					t.Errorf("buildNonmemBinaryPath() = %q, unexpected backslashes on Unix system", result)
				}
			}
			// Skip validation for cross-platform test cases
		})
	}
}

func TestBuildNonmemBinaryPathSpecificScenarios(t *testing.T) {
	t.Run("Windows C drive path", func(t *testing.T) {
		result, err := buildNonmemBinaryPath("C:/NONMEM", "nmfe76")
		if err != nil {
			t.Errorf("buildNonmemBinaryPath() returned error: %v", err)
			return
		}

		// Should contain the basic components
		if !strings.Contains(result, "NONMEM") {
			t.Errorf("Expected result to contain 'NONMEM', got: %s", result)
		}
		if !strings.Contains(result, "nmfe76") {
			t.Errorf("Expected result to contain 'nmfe76', got: %s", result)
		}

		// On Windows, should be absolute
		if runtime.GOOS == "windows" && !filepath.IsAbs(result) {
			t.Errorf("Expected absolute path on Windows, got: %s", result)
		}
	})

	t.Run("Linux opt path", func(t *testing.T) {
		result, err := buildNonmemBinaryPath("/opt/NONMEM", "nmfe76")
		if err != nil {
			t.Errorf("buildNonmemBinaryPath() returned error: %v", err)
			return
		}

		// Should contain the basic components
		if !strings.Contains(result, "opt") {
			t.Errorf("Expected result to contain 'opt', got: %s", result)
		}
		if !strings.Contains(result, "NONMEM") {
			t.Errorf("Expected result to contain 'NONMEM', got: %s", result)
		}
		if !strings.Contains(result, "nmfe76") {
			t.Errorf("Expected result to contain 'nmfe76', got: %s", result)
		}

		// Should be absolute (starts with /)
		if !filepath.IsAbs(result) {
			t.Errorf("Expected absolute path, got: %s", result)
		}
	})

	t.Run("Path joining behavior", func(t *testing.T) {
		// Test that the function properly joins paths
		result, err := buildNonmemBinaryPath("path1", "path2")
		if err != nil {
			t.Errorf("buildNonmemBinaryPath() returned error: %v", err)
			return
		}

		// The result should at least contain what filepath.Join would produce
		// (it might be an absolute version of it)
		if !strings.Contains(result, "path1") || !strings.Contains(result, "path2") {
			t.Errorf("buildNonmemBinaryPath() should join paths properly, got: %s", result)
		}
	})

	t.Run("Error handling", func(t *testing.T) {
		// Test with an invalid path that might cause filepath.Abs to fail
		// Note: filepath.Abs rarely fails in practice, but we test the error handling mechanism
		_, err := buildNonmemBinaryPath("", "")

		// We don't expect an error for empty strings (they're valid input)
		// but this tests that our error handling works correctly
		if err != nil && !strings.Contains(err.Error(), "failed to resolve absolute path") {
			t.Errorf("Expected specific error message format, got: %v", err)
		}
	})
}
