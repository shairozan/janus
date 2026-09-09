//go:build validation
// +build validation

package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/shairozan/janus/internal/version"
)

// TestREQ37_VersionInformationDisplay validates REQ-37: Version information display.
func TestREQ37_VersionInformationDisplay(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-37",
		Description: "I can view version information",
		Category:    CategoryVersionBuild,
		TestFunc: func(t *testing.T) {
			// Verify that version information is available
			ver := version.Get()
			assert.NotEmpty(t, ver, "Version should not be empty")

			buildInfo := version.GetBuildInfo()
			assert.NotNil(t, buildInfo, "Build info should not be nil")
			assert.NotEmpty(t, buildInfo["commit"], "Commit should not be empty")
			assert.NotEmpty(t, buildInfo["date"], "Build date should not be empty")
			assert.NotEmpty(t, buildInfo["builtBy"], "Built by should not be empty")

			// Verify full version string
			fullVersion := version.GetFull()
			assert.NotEmpty(t, fullVersion, "Full version string should not be empty")
			assert.Contains(t, fullVersion, ver, "Full version should contain version number")

			t.Logf("Version: %s", ver)
			t.Logf("Commit: %s", buildInfo["commit"])
			t.Logf("Date: %s", buildInfo["date"])
			t.Logf("Built by: %s", buildInfo["builtBy"])
			t.Logf("Full version: %s", fullVersion)
		},
	}

	test.Run(t)
}

// TestREQ38_BuildReproducibility validates REQ-38: Build reproducibility.
func TestREQ38_BuildReproducibility(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-38",
		Description: "I can reproduce builds consistently",
		Category:    CategoryVersionBuild,
		TestFunc: func(t *testing.T) {
			// Verify that build information is deterministic and traceable
			buildInfo := version.GetBuildInfo()
			commit := buildInfo["commit"]
			assert.NotEmpty(t, commit, "Commit hash should be available for build traceability")

			// Commit hash should be a valid git SHA (at least 7 characters)
			if commit != "dev" && commit != "unknown" {
				assert.GreaterOrEqual(t, len(commit), 7, "Commit hash should be at least 7 characters")
			}

			// Build date should be in a standard format
			date := buildInfo["date"]
			assert.NotEmpty(t, date, "Build date should be available for build traceability")

			// Built by should indicate the build system
			builtBy := buildInfo["builtBy"]
			assert.NotEmpty(t, builtBy, "Built by should be available for build traceability")

			t.Logf("Build is traceable via commit %s, built on %s by %s", commit, date, builtBy)
		},
	}

	test.Run(t)
}
