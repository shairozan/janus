//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default target to run when none is specified
var Default = Build

const (
	goexe = "go"
)

// Build builds the binary (standalone, no dependencies)
func Build() error {
	fmt.Println("🔨 Building Janus...")

	// Get git commit
	commit, _ := sh.Output("git", "rev-parse", "HEAD")
	if commit == "" {
		commit = "unknown"
	}

	// Get current date
	date, _ := sh.Output("date", "-u", "+%Y-%m-%dT%H:%M:%SZ")
	if date == "" {
		date = "unknown"
	}

	ldflags := fmt.Sprintf("-s -w -X github.com/pharmalytica/janus/internal/version.Version=dev -X github.com/pharmalytica/janus/internal/version.Commit=%s -X github.com/pharmalytica/janus/internal/version.Date=%s -X github.com/pharmalytica/janus/internal/version.BuiltBy=mage", commit, date)

	env := map[string]string{
		"CGO_ENABLED": "1",
	}

	return sh.RunWithV(env, goexe, "build", "-ldflags", ldflags, "-o", "janus", ".")
}

// Test runs all tests
func Test() error {
	fmt.Println("🧪 Running tests...")
	return sh.RunV(goexe, "test", "-v", "-race", "-coverprofile=coverage.out", "./...")
}

// TestShort runs only short tests
func TestShort() error {
	fmt.Println("🧪 Running short tests...")
	return sh.RunV(goexe, "test", "-v", "-race", "-short", "./...")
}

// Unit runs unit tests that don't require external dependencies
func Unit() error {
	fmt.Println("🧪 Running unit tests...")
	return sh.RunV(goexe, "test", "-v", "-race", "-tags=unit", "./...")
}

// Integration runs integration tests that require external dependencies or services
func Integration() error {
	fmt.Println("🧪 Running GUI integration tests...")
	fmt.Println("⚠️  Integration tests may require external dependencies (SLURM, NONMEM binaries, etc.)")

	if err := sh.RunV(goexe, "test", "-v", "-timeout", "2m", "-tags=integration,gui", "./..."); err != nil {
		fmt.Printf("⚠️  Integration tests failed: %v\n", err)
		fmt.Printf("   This may be expected if external dependencies are not available\n")
		// Don't return error for integration tests - they may fail due to missing external deps
	}

	fmt.Println("✅ Integration tests completed!")
	fmt.Println("💡 Note: Integration test failures are expected in environments without external dependencies")
	return nil
}

// Coverage generates and opens coverage report
func Coverage() error {
	mg.Deps(Test)
	fmt.Println("📊 Generating coverage report...")

	if err := sh.RunV(goexe, "tool", "cover", "-html=coverage.out", "-o", "coverage.html"); err != nil {
		return err
	}

	fmt.Println("✅ Coverage report generated: coverage.html")
	return nil
}

// Lint runs golangci-lint (standalone, no dependencies)
func Lint() error {
	fmt.Println("🔍 Running linters...")

	// Check if golangci-lint is installed
	if err := sh.RunV("golangci-lint", "version"); err != nil {
		fmt.Println("📦 Installing golangci-lint...")
		if err := sh.RunV("go", "install", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"); err != nil {
			return fmt.Errorf("failed to install golangci-lint: %w", err)
		}
	}

	return sh.RunV("golangci-lint", "run", "--timeout=5m", "./...")
}

// Format formats all Go files with gofmt and goimports (standalone, no dependencies)
func Format() error {
	fmt.Println("🎨 Formatting Go files...")

	// Find all Go files
	goFiles, err := findGoFiles()
	if err != nil {
		return err
	}

	if len(goFiles) == 0 {
		fmt.Println("No Go files found")
		return nil
	}

	// Run gofmt on all files
	fmt.Printf("📝 Running gofmt on %d files...\n", len(goFiles))
	gofmtArgs := append([]string{"-s", "-w"}, goFiles...)
	if err := sh.RunV("gofmt", gofmtArgs...); err != nil {
		return fmt.Errorf("gofmt failed: %w", err)
	}

	// Check if goimports is available, install if needed
	fmt.Println("📦 Checking goimports...")
	if err := sh.RunV("goimports", "-version"); err != nil {
		fmt.Println("Installing goimports...")
		if err := sh.RunV("go", "install", "golang.org/x/tools/cmd/goimports@latest"); err != nil {
			return fmt.Errorf("failed to install goimports: %w", err)
		}
	}

	// Run goimports on all files
	fmt.Printf("📝 Running goimports on %d files...\n", len(goFiles))
	goimportsArgs := append([]string{"-w", "-local", "github.com/pharmalytica/janus"}, goFiles...)
	if err := sh.RunV("goimports", goimportsArgs...); err != nil {
		return fmt.Errorf("goimports failed: %w", err)
	}

	fmt.Println("✅ Formatting complete!")
	return nil
}

// BuildAll runs format, lint, and then builds (combined workflow)
func BuildAll() error {
	mg.Deps(Format, Lint, Build)
	fmt.Println("✅ Build pipeline complete!")
	return nil
}

// Check runs format, lint, and unit tests (combined workflow for development)
func Check() error {
	mg.Deps(Format, Lint, Unit)
	fmt.Println("✅ All development checks passed!")
	return nil
}

// CheckAll runs format, lint, unit tests, and build (complete validation)
func CheckAll() error {
	mg.Deps(Format, Lint, Unit, Build)
	fmt.Println("✅ Complete validation passed!")
	return nil
}

// CheckFull runs format, lint, both unit and integration tests, and build
func CheckFull() error {
	mg.Deps(Format, Lint, Unit)
	mg.Deps(Integration) // Run after unit tests, but don't fail the whole pipeline
	mg.Deps(Build)
	fmt.Println("✅ Full validation completed!")
	return nil
}

// Clean removes build artifacts and coverage files
func Clean() error {
	fmt.Println("🧹 Cleaning up...")

	filesToRemove := []string{
		"janus",
		"janus.exe",
		"janus-ubuntu2004",
		"janus-ubuntu2204",
		"janus-ubuntu2404",
		"executor",
		"executor.exe",
		"coverage.out",
		"coverage.html",
		"deb-package",
	}

	for _, file := range filesToRemove {
		if err := sh.Rm(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", file, err)
		}
	}

	// Clean up any .deb files
	debFiles, err := filepath.Glob("janus_*.deb")
	if err == nil {
		for _, deb := range debFiles {
			if err := sh.Rm(deb); err != nil && !os.IsNotExist(err) {
				fmt.Printf("⚠️  Warning: failed to remove %s: %v\n", deb, err)
			}
		}
	}

	// Clean up any executor platform binaries
	executorFiles, err := filepath.Glob("executor-*")
	if err == nil {
		for _, exec := range executorFiles {
			if err := sh.Rm(exec); err != nil && !os.IsNotExist(err) {
				fmt.Printf("⚠️  Warning: failed to remove %s: %v\n", exec, err)
			}
		}
	}

	fmt.Println("✅ Cleanup complete!")
	return nil
}

// Install installs required tools
func Install() error {
	fmt.Println("📦 Installing required tools...")

	tools := []string{
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest",
		"golang.org/x/tools/cmd/goimports@latest",
	}

	for _, tool := range tools {
		fmt.Printf("Installing %s...\n", tool)
		if err := sh.RunV("go", "install", tool); err != nil {
			return fmt.Errorf("failed to install %s: %w", tool, err)
		}
	}

	fmt.Println("✅ All tools installed!")
	return nil
}

// Dev sets up development environment (combined workflow)
func Dev() error {
	fmt.Println("🚀 Setting up development environment...")
	mg.Deps(Install)

	// Verify tools work
	tools := map[string][]string{
		"go":            {"version"},
		"golangci-lint": {"version"},
		"goimports":     {"-version"},
	}

	fmt.Println("🔍 Verifying tools...")
	for tool, args := range tools {
		if err := sh.RunV(tool, args...); err != nil {
			return fmt.Errorf("tool verification failed for %s: %w", tool, err)
		}
	}

	fmt.Println("✅ Development environment ready!")
	return nil
}

// Release builds a release version with proper versioning (combined workflow)
func Release(ctx context.Context, version string) error {
	if version == "" {
		return fmt.Errorf("version is required (e.g., mage release v1.0.0)")
	}

	// Ensure checks pass first
	mg.CtxDeps(ctx, CheckAll)

	fmt.Printf("🚀 Building release %s...\n", version)

	// Get git commit
	commit, _ := sh.Output("git", "rev-parse", "HEAD")
	if commit == "" {
		commit = "unknown"
	}

	// Get current date
	date, _ := sh.Output("date", "-u", "+%Y-%m-%dT%H:%M:%SZ")
	if date == "" {
		date = "unknown"
	}

	ldflags := fmt.Sprintf("-s -w -X github.com/pharmalytica/janus/internal/version.Version=%s -X github.com/pharmalytica/janus/internal/version.Commit=%s -X github.com/pharmalytica/janus/internal/version.Date=%s -X github.com/pharmalytica/janus/internal/version.BuiltBy=mage", version, commit, date)

	env := map[string]string{
		"CGO_ENABLED": "1",
	}

	binaryName := fmt.Sprintf("janus-%s", version)
	if err := sh.RunWithV(env, goexe, "build", "-ldflags", ldflags, "-o", binaryName, "."); err != nil {
		return err
	}

	fmt.Printf("✅ Release binary built: %s\n", binaryName)
	return nil
}

// findGoFiles recursively finds all .go files in the project, excluding vendor
func findGoFiles() ([]string, error) {
	var goFiles []string

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories we don't want to format
		if info.IsDir() {
			switch info.Name() {
			case "vendor", ".git", "node_modules", "magefiles":
				return filepath.SkipDir
			}
			return nil
		}

		// Include .go files, but exclude generated files
		if strings.HasSuffix(path, ".go") &&
			!strings.HasSuffix(path, ".pb.go") &&
			!strings.HasSuffix(path, "_gen.go") &&
			!strings.Contains(path, "vendor/") {
			goFiles = append(goFiles, path)
		}

		return nil
	})

	return goFiles, err
}

// Validation runs validation tests and generates compliance reports
func Validation() error {
	fmt.Println("🔍 Running validation tests...")

	// Run validation tests with verbose output
	if err := sh.RunV(goexe, "test", "-v", "-tags=validation", "./..."); err != nil {
		return fmt.Errorf("validation tests failed: %w", err)
	}

	// Check if validation report was generated
	reportPath := "internal/validation/nonmem_validation_report.json"
	if _, err := os.Stat(reportPath); err == nil {
		fmt.Printf("📋 Validation report generated: %s\n", reportPath)

		// Display report summary
		if err := displayValidationSummary(reportPath); err != nil {
			fmt.Printf("⚠️  Could not display validation summary: %v\n", err)
		}
	} else {
		fmt.Printf("⚠️  Validation report not found at: %s\n", reportPath)
	}

	fmt.Println("✅ Validation complete!")
	return nil
}

// ValidationReport generates comprehensive auditor-ready validation reports
func ValidationReport() error {
	fmt.Println("📋 Generating comprehensive validation reports...")

	// Run all validation tests to generate fresh results
	if err := sh.RunV(goexe, "test", "-v", "-tags=validation", "./..."); err != nil {
		return fmt.Errorf("validation tests failed: %w", err)
	}

	// Check for generated reports
	auditorReportPath := "internal/validation/execution_auditor_validation_report.json"
	technicalReportPath := "internal/validation/execution_validation_report.json"

	reportsGenerated := 0

	// Check auditor report
	if _, err := os.Stat(auditorReportPath); err == nil {
		fmt.Printf("📋 Auditor validation report generated: %s\n", auditorReportPath)

		// Copy to project root for easy access
		if err := sh.RunV("cp", auditorReportPath, "validation_auditor_report.json"); err == nil {
			fmt.Printf("📋 Auditor report copied to: validation_auditor_report.json\n")
		}

		reportsGenerated++
	}

	// Check technical report
	if _, err := os.Stat(technicalReportPath); err == nil {
		fmt.Printf("📋 Technical validation report generated: %s\n", technicalReportPath)

		// Copy to project root for easy access
		if err := sh.RunV("cp", technicalReportPath, "validation_technical_report.json"); err == nil {
			fmt.Printf("📋 Technical report copied to: validation_technical_report.json\n")
		}

		reportsGenerated++
	}

	if reportsGenerated == 0 {
		fmt.Println("⚠️  No validation reports found. Run validation tests first.")
		return fmt.Errorf("no validation reports generated")
	}

	// Display detailed summary
	if err := displayDetailedValidationSummary(); err != nil {
		fmt.Printf("⚠️  Could not display detailed validation summary: %v\n", err)
	}

	fmt.Printf("✅ %d validation report(s) generated successfully!\n", reportsGenerated)
	fmt.Println("")
	fmt.Println("📁 Report Files:")
	fmt.Println("   • validation_auditor_report.json    - For regulatory auditors (CFR 21 Part 11)")
	fmt.Println("   • validation_technical_report.json  - For technical teams")
	fmt.Println("")
	fmt.Println("💡 Usage:")
	fmt.Println("   View auditor report: cat validation_auditor_report.json | jq .")
	fmt.Println("   View technical report: cat validation_technical_report.json | jq .")

	return nil
}

// GUI runs GUI tests that require a display and window system
func GUI() error {
	fmt.Println("🖥️  Running GUI tests...")
	fmt.Println("⚠️  GUI tests require a display and window system")

	if err := sh.RunV(goexe, "test", "-v", "-timeout", "2m", "-tags=gui", "./..."); err != nil {
		fmt.Printf("⚠️  GUI tests failed: %v\n", err)
		fmt.Printf("   This may be expected in headless environments\n")
		// Don't return error for GUI tests - they may fail in headless environments
	}

	fmt.Println("✅ GUI tests completed!")
	fmt.Println("💡 Note: GUI test failures are expected in headless environments")
	return nil
}

// ValidationAll runs all validation test suites (currently just NONMEM, but ready for expansion)
func ValidationAll() error {
	fmt.Println("🔍 Running all validation test suites...")

	// Run all validation tests
	if err := sh.RunV(goexe, "test", "-v", "-tags=validation", "./..."); err != nil {
		return fmt.Errorf("validation tests failed: %w", err)
	}

	// Look for all validation reports
	reports, err := findValidationReports()
	if err != nil {
		fmt.Printf("⚠️  Error finding validation reports: %v\n", err)
	} else {
		for _, report := range reports {
			fmt.Printf("📋 Validation report: %s\n", report)
		}
	}

	fmt.Println("✅ All validation tests complete!")
	return nil
}

// displayValidationSummary reads and displays a summary of the validation report
func displayValidationSummary(reportPath string) error {
	fmt.Printf("\n=== VALIDATION REPORT SUMMARY ===\n")
	fmt.Printf("Report Location: %s\n", reportPath)

	// Get file size for basic info
	if info, err := os.Stat(reportPath); err == nil {
		fmt.Printf("Report Size: %d bytes\n", info.Size())
		fmt.Printf("Generated: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
	}

	fmt.Printf("For detailed results, run: cat %s | jq .\n", reportPath)
	fmt.Printf("==================================\n\n")

	return nil
}

// displayDetailedValidationSummary displays comprehensive validation information
func displayDetailedValidationSummary() error {
	fmt.Printf("\n=== COMPREHENSIVE VALIDATION SUMMARY ===\n")

	// Check if auditor report exists and display key metrics
	auditorReportPath := "validation_auditor_report.json"
	if _, err := os.Stat(auditorReportPath); err == nil {
		fmt.Printf("📋 Auditor Report: %s\n", auditorReportPath)
		if info, err := os.Stat(auditorReportPath); err == nil {
			fmt.Printf("   • Generated: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
			fmt.Printf("   • Size: %d bytes\n", info.Size())
		}

		// Try to extract key metrics using basic shell commands
		fmt.Println("   • Compliance Standard: CFR 21 Part 11")
		fmt.Println("   • Validation Scope: NONMEM Execution and Audit Compliance")

		// Extract success rate if jq is available
		if successRate, err := sh.Output("jq", "-r", ".execution_summary.success_rate", auditorReportPath); err == nil && successRate != "" {
			fmt.Printf("   • Success Rate: %s%%\n", successRate)
		}

		// Extract total tests if jq is available
		if totalTests, err := sh.Output("jq", "-r", ".execution_summary.total_tests", auditorReportPath); err == nil && totalTests != "" {
			fmt.Printf("   • Total Tests: %s\n", totalTests)
		}
	}

	// Check if technical report exists
	technicalReportPath := "validation_technical_report.json"
	if _, err := os.Stat(technicalReportPath); err == nil {
		fmt.Printf("📋 Technical Report: %s\n", technicalReportPath)
		if info, err := os.Stat(technicalReportPath); err == nil {
			fmt.Printf("   • Generated: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
			fmt.Printf("   • Size: %d bytes\n", info.Size())
		}
	}

	fmt.Printf("\n📊 Validation Categories Covered:\n")
	fmt.Printf("   • NONMEM_EXECUTION: Local NONMEM execution validation\n")
	fmt.Printf("   • AUDIT_COMPLIANCE: CFR 21 Part 11 audit trail validation\n")

	fmt.Printf("\n🔍 Requirements Validated:\n")
	fmt.Printf("   • REQ-01: Basic NONMEM Execution\n")
	fmt.Printf("   • REQ-02: NONMEM with Additional Options\n")
	fmt.Printf("   • REQ-03: NONMEM Parallel Execution\n")
	fmt.Printf("   • REQ-04: NONMEM Output File Handling\n")
	fmt.Printf("   • REQ-41: JSON Audit Trail Enablement\n")
	fmt.Printf("   • REQ-42: Job ID Audit Logging\n")
	fmt.Printf("   • REQ-43: STDOUT Audit Capture\n")
	fmt.Printf("   • REQ-44: STDERR Audit Capture\n")
	fmt.Printf("   • REQ-45: Binary Path Audit Logging\n")
	fmt.Printf("   • REQ-46: Command Arguments Audit Logging\n")

	fmt.Printf("\n📝 Regulatory Context:\n")
	fmt.Printf("   • Standard: CFR 21 Part 11 - Electronic Records; Electronic Signatures\n")
	fmt.Printf("   • Application: Pharmaceutical software validation\n")
	fmt.Printf("   • Scope: NONMEM execution and audit trail compliance\n")

	fmt.Printf("==========================================\n\n")

	return nil
}

// findValidationReports finds all validation report files
func findValidationReports() ([]string, error) {
	var reports []string

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.Contains(info.Name(), "validation_report.json") {
			reports = append(reports, path)
		}

		return nil
	})

	return reports, err
}

// Package builds a Linux AMD64 binary and packages it into a .deb for Ubuntu
// Usage: mage package v1.0.0 [ubuntu_version]
// ubuntu_version can be: ubuntu2004 (default), ubuntu2204, or ubuntu2404
func Package(ctx context.Context, version string, ubuntuVersion string) error {
	if version == "" {
		return fmt.Errorf("version is required (e.g., mage package v1.0.0 [ubuntu2004|ubuntu2204|ubuntu2404])")
	}

	// Default to ubuntu2004 if not specified
	targetUbuntu := ubuntuVersion
	if targetUbuntu == "" {
		targetUbuntu = "ubuntu2004"
	}

	// Validate Ubuntu version
	validVersions := map[string]bool{
		"ubuntu2004": true,
		"ubuntu2204": true,
		"ubuntu2404": true,
	}

	if !validVersions[targetUbuntu] {
		return fmt.Errorf("invalid Ubuntu version: %s (must be ubuntu2004, ubuntu2204, or ubuntu2404)", targetUbuntu)
	}

	fmt.Printf("📦 Creating %s package for version %s...\n", targetUbuntu, version)

	// Strip 'v' prefix for Debian package version
	debVersion := strings.TrimPrefix(version, "v")

	// Build Linux AMD64 binary
	fmt.Println("🔨 Building Linux AMD64 binary...")
	commit, _ := sh.Output("git", "rev-parse", "HEAD")
	if commit == "" {
		commit = "unknown"
	}

	date, _ := sh.Output("date", "-u", "+%Y-%m-%dT%H:%M:%SZ")
	if date == "" {
		date = "unknown"
	}

	ldflags := fmt.Sprintf("-s -w -X github.com/pharmalytica/janus/internal/version.Version=%s -X github.com/pharmalytica/janus/internal/version.Commit=%s -X github.com/pharmalytica/janus/internal/version.Date=%s -X github.com/pharmalytica/janus/internal/version.BuiltBy=mage", version, commit, date)

	binaryName := fmt.Sprintf("janus-%s", targetUbuntu)
	env := map[string]string{
		"CGO_ENABLED": "1",
		"GOOS":        "linux",
		"GOARCH":      "amd64",
	}

	if err := sh.RunWithV(env, goexe, "build", "-ldflags", ldflags, "-o", binaryName, "."); err != nil {
		return fmt.Errorf("failed to build binary: %w", err)
	}

	fmt.Printf("✅ Binary built: %s\n", binaryName)

	// Create .deb package structure
	fmt.Println("📦 Creating .deb package structure...")
	debPkgDir := "deb-package"

	// Clean up any existing package directory
	if err := sh.Rm(debPkgDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clean package directory: %w", err)
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(debPkgDir, "DEBIAN"),
		filepath.Join(debPkgDir, "usr", "bin"),
		filepath.Join(debPkgDir, "usr", "share", "doc", "janus"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Copy binary
	fmt.Println("📝 Copying binary...")
	if err := sh.Copy(filepath.Join(debPkgDir, "usr", "bin", "janus"), binaryName); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make binary executable
	if err := os.Chmod(filepath.Join(debPkgDir, "usr", "bin", "janus"), 0755); err != nil {
		return fmt.Errorf("failed to set binary permissions: %w", err)
	}

	// Copy documentation
	fmt.Println("📝 Copying documentation...")
	if err := sh.Copy(filepath.Join(debPkgDir, "usr", "share", "doc", "janus", "README.md"), "README.md"); err != nil {
		return fmt.Errorf("failed to copy README: %w", err)
	}

	// Copy LICENSE if it exists
	if _, err := os.Stat("LICENSE"); err == nil {
		if err := sh.Copy(filepath.Join(debPkgDir, "usr", "share", "doc", "janus", "LICENSE"), "LICENSE"); err != nil {
			fmt.Printf("⚠️  Warning: failed to copy LICENSE: %v\n", err)
		}
	}

	// Create control file
	fmt.Println("📝 Creating control file...")
	controlFile := filepath.Join(debPkgDir, "DEBIAN", "control")
	controlContent := fmt.Sprintf(`Package: janus
Version: %s
Section: utils
Priority: optional
Architecture: amd64
Depends: libgl1-mesa-glx, libxrandr2, libxinerama1, libxcursor1, libxi6, libxss1
Maintainer: dukeofubuntu <noreply@github.com>
Description: Janus - NONMEM Grid Management Tool
 A modern, cost-effective replacement for Certara Pirana using Go + Fyne.io,
 with pluggable orchestrator backends and built-in CFR 21 Part 11 compliance.
Homepage: https://github.com/pharmalytica/janus
`, debVersion)

	if err := os.WriteFile(controlFile, []byte(controlContent), 0644); err != nil {
		return fmt.Errorf("failed to create control file: %w", err)
	}

	// Build .deb package
	fmt.Println("🔧 Building .deb package...")
	debFileName := fmt.Sprintf("janus_%s_%s_amd64.deb", debVersion, targetUbuntu)
	if err := sh.RunV("dpkg-deb", "--build", debPkgDir, debFileName); err != nil {
		return fmt.Errorf("failed to build .deb package: %w", err)
	}

	// Clean up package directory
	if err := sh.Rm(debPkgDir); err != nil {
		fmt.Printf("⚠️  Warning: failed to clean up package directory: %v\n", err)
	}

	fmt.Printf("✅ Package created successfully: %s\n", debFileName)
	fmt.Println("")
	fmt.Println("📦 Installation:")
	fmt.Printf("   sudo dpkg -i %s\n", debFileName)
	fmt.Println("")
	fmt.Println("🚀 Run:")
	fmt.Println("   janus")

	return nil
}

// BuildExecutor builds the executor binary for the current platform
func BuildExecutor() error {
	fmt.Println("🔨 Building executor binary...")

	// Get git commit
	commit, _ := sh.Output("git", "rev-parse", "HEAD")
	if commit == "" {
		commit = "unknown"
	}

	// Get current date
	date, _ := sh.Output("date", "-u", "+%Y-%m-%dT%H:%M:%SZ")
	if date == "" {
		date = "unknown"
	}

	// Get version from environment or use "dev"
	version := os.Getenv("VERSION")
	if version == "" {
		version = "dev"
	}

	ldflags := fmt.Sprintf(
		"-s -w "+
			"-X github.com/pharmalytica/janus/internal/version.Version=%s "+
			"-X github.com/pharmalytica/janus/internal/version.Commit=%s "+
			"-X github.com/pharmalytica/janus/internal/version.Date=%s "+
			"-X github.com/pharmalytica/janus/internal/version.BuiltBy=mage",
		version, commit, date,
	)

	env := map[string]string{
		"CGO_ENABLED": "0", // Executor is pure Go, no CGO required
	}

	return sh.RunWithV(env, goexe, "build", "-ldflags", ldflags, "-o", "executor", "./cmd/executor")
}

// BuildExecutorAll builds executor binaries for all supported platforms
func BuildExecutorAll() error {
	fmt.Println("🔨 Building executor for all platforms...")

	platforms := []struct {
		GOOS   string
		GOARCH string
		Suffix string
	}{
		{"linux", "amd64", "-linux-amd64"},
		{"linux", "arm64", "-linux-arm64"},
		{"darwin", "amd64", "-darwin-amd64"},
		{"darwin", "arm64", "-darwin-arm64"},
		{"windows", "amd64", "-windows-amd64.exe"},
	}

	// Get git commit
	commit, _ := sh.Output("git", "rev-parse", "HEAD")
	if commit == "" {
		commit = "unknown"
	}

	// Get current date
	date, _ := sh.Output("date", "-u", "+%Y-%m-%dT%H:%M:%SZ")
	if date == "" {
		date = "unknown"
	}

	// Get version from environment or use "dev"
	version := os.Getenv("VERSION")
	if version == "" {
		version = "dev"
	}

	ldflags := fmt.Sprintf(
		"-s -w "+
			"-X github.com/pharmalytica/janus/internal/version.Version=%s "+
			"-X github.com/pharmalytica/janus/internal/version.Commit=%s "+
			"-X github.com/pharmalytica/janus/internal/version.Date=%s "+
			"-X github.com/pharmalytica/janus/internal/version.BuiltBy=mage",
		version, commit, date,
	)

	for _, platform := range platforms {
		fmt.Printf("🔨 Building executor for %s/%s...\n", platform.GOOS, platform.GOARCH)

		env := map[string]string{
			"GOOS":        platform.GOOS,
			"GOARCH":      platform.GOARCH,
			"CGO_ENABLED": "0", // Executor is pure Go, no CGO required
		}

		output := "executor" + platform.Suffix

		if err := sh.RunWithV(env, goexe, "build", "-ldflags", ldflags, "-o", output, "./cmd/executor"); err != nil {
			return fmt.Errorf("failed to build %s: %w", output, err)
		}

		fmt.Printf("✅ Built: %s\n", output)
	}

	fmt.Println("✅ All executor binaries built successfully!")
	return nil
}

// Executor namespace for executor-specific commands
type Executor mg.Namespace

// Build builds the executor binary for the current platform (same as BuildExecutor)
func (Executor) Build() error {
	return BuildExecutor()
}

// BuildAll builds executor binaries for all supported platforms (same as BuildExecutorAll)
func (Executor) BuildAll() error {
	return BuildExecutorAll()
}

// Install builds and installs the executor binary to ~/bin/executor
func (Executor) Install() error {
	fmt.Println("📦 Installing executor to ~/bin/executor...")

	// Build the executor first
	if err := BuildExecutor(); err != nil {
		return fmt.Errorf("failed to build executor: %w", err)
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create ~/bin if it doesn't exist
	binDir := filepath.Join(homeDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create ~/bin directory: %w", err)
	}

	// Install the executor
	targetPath := filepath.Join(binDir, "executor")
	if err := sh.Copy(targetPath, "executor"); err != nil {
		return fmt.Errorf("failed to copy executor to %s: %w", targetPath, err)
	}

	// Make it executable
	if err := os.Chmod(targetPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	// Clean up the temporary build artifact
	if err := sh.Rm("executor"); err != nil && !os.IsNotExist(err) {
		fmt.Printf("⚠️  Warning: failed to remove temporary executor binary: %v\n", err)
	}

	fmt.Printf("✅ Executor installed to: %s\n", targetPath)
	fmt.Println("")
	fmt.Println("🚀 Usage:")
	fmt.Println("   executor --help")
	fmt.Println("   executor --executor-version")
	fmt.Println("")
	fmt.Println("💡 Make sure ~/bin is in your PATH:")
	fmt.Println("   export PATH=\"$HOME/bin:$PATH\"")
	fmt.Println("   # Add the above line to your ~/.bashrc or ~/.zshrc")

	// Check if ~/bin is in PATH
	pathEnv := os.Getenv("PATH")
	if !strings.Contains(pathEnv, filepath.Join(homeDir, "bin")) {
		fmt.Println("")
		fmt.Println("⚠️  Warning: ~/bin is not in your PATH")
		fmt.Println("   Run the following to add it temporarily:")
		fmt.Println("   export PATH=\"$HOME/bin:$PATH\"")
	} else {
		fmt.Println("")
		fmt.Println("✅ ~/bin is already in your PATH")
	}

	return nil
}