//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Docker contains targets for Docker-based operations
type Docker mg.Namespace

const (
	dockerImage    = "dukeofubuntu/janus-ci:latest-ubuntu24"
	devDockerImage = "janus-dev:latest"
)

// runInDevContainer runs a command in the development Docker container
// If the dev image doesn't exist, it provides helpful instructions
func getCurrentDir() string {
	pwd, _ := os.Getwd()
	return pwd
}

func runInDevContainer(args ...string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if dev image exists
	if err := sh.RunV("docker", "image", "inspect", devDockerImage); err != nil {
		fmt.Printf("❌ Development image '%s' not found\n", devDockerImage)
		fmt.Println("💡 Run 'mage docker:buildDev' first to create the development image")
		return fmt.Errorf("development image not found")
	}

	// Prepare docker run arguments
	dockerArgs := []string{"run", "--rm", "-v", pwd + ":/workspace", "-w", "/workspace"}

	// Add display for GUI tests if needed
	for _, arg := range args {
		if arg == "gui" {
			dockerArgs = append(dockerArgs, "-e", "DISPLAY=:99")
			break
		}
	}

	dockerArgs = append(dockerArgs, devDockerImage)
	dockerArgs = append(dockerArgs, args...)

	return sh.RunV("docker", dockerArgs...)
}

// Build builds the binary using the development Docker image (fast with cached dependencies)
func (Docker) Build() error {
	fmt.Println("🐳 Building Janus using development Docker image...")
	return runInDevContainer("mage", "build")
}

// Test runs all tests using the development Docker image (fast with cached dependencies)
func (Docker) Test() error {
	fmt.Println("🐳 Running tests using development Docker image...")
	return runInDevContainer("mage", "test")
}

// Unit runs unit tests using the development Docker image (fast with cached dependencies)
func (Docker) Unit() error {
	fmt.Println("🐳 Running unit tests using development Docker image...")
	return runInDevContainer("go", "test", "-v", "-race", "-tags=unit", "./...")
}

// Integration runs GUI integration tests using the development Docker image (fast with cached dependencies)
func (Docker) Integration() error {
	fmt.Println("🐳 Running GUI integration tests using development Docker image...")
	return runInDevContainer("go", "test", "-v", "-timeout", "60s", "-tags=integration,gui", "./...")
}

// Lint runs linting using the development Docker image (fast with cached dependencies)
func (Docker) Lint() error {
	fmt.Println("🐳 Running linters using development Docker image...")
	return runInDevContainer("mage", "lint")
}

// Check runs format, lint, and unit tests using the development Docker image (fast with cached dependencies)
func (Docker) Check() error {
	fmt.Println("🐳 Running development checks using development Docker image...")
	return runInDevContainer("mage", "check")
}

// CheckAll runs format, lint, unit tests, and build using the development Docker image (fast complete validation)
func (Docker) CheckAll() error {
	fmt.Println("🐳 Running complete validation using development Docker image...")
	return runInDevContainer("mage", "checkall")
}

// SignalTest runs the signal handling tests in Docker
func (Docker) SignalTest() error {
	fmt.Println("🐳 Running signal handling tests in Docker...")

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	return sh.RunV("docker", "run", "--rm",
		"-v", pwd+":/workspace",
		"-w", "/workspace",
		dockerImage,
		"go", "test", "-v", "./internal/execution/", "-run", "TestSignalHandling|TestEndToEndSignalHandling|TestContextCancellation")
}

// MockSlurm runs the mock SLURM tests in Docker
func (Docker) MockSlurm() error {
	fmt.Println("🐳 Running mock SLURM tests in Docker...")

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	return sh.RunV("docker", "run", "--rm",
		"-v", pwd+":/workspace",
		"-w", "/workspace",
		dockerImage,
		"go", "test", "-v", "./internal/execution/", "-run", "TestSLURMExecutorWithMockClient")
}

// Validation runs validation tests using the development Docker image (fast with cached dependencies)
func (Docker) Validation() error {
	fmt.Println("🐳 Running validation tests using development Docker image...")
	return runInDevContainer("go", "test", "-v", "-tags=validation", "./...")
}

// GUI runs GUI tests using the development Docker image with display support (fast with cached dependencies)
func (Docker) GUI() error {
	fmt.Println("🐳 Running GUI tests using development Docker image...")

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if dev image exists
	if err := sh.RunV("docker", "image", "inspect", devDockerImage); err != nil {
		fmt.Printf("❌ Development image '%s' not found\n", devDockerImage)
		fmt.Println("💡 Run 'mage docker:buildDev' first to create the development image")
		return fmt.Errorf("development image not found")
	}

	return sh.RunV("docker", "run", "--rm",
		"-v", pwd+":/workspace",
		"-w", "/workspace",
		"-e", "DISPLAY=:99",
		devDockerImage,
		"go", "test", "-v", "-timeout", "60s", "-tags=gui", "./...")
}

// BuildDev builds the local development Docker image with cached Go modules
func (Docker) BuildDev() error {
	fmt.Println("🐳 Building local development Docker image...")

	fmt.Println("📦 This will cache Go modules for faster subsequent builds")
	fmt.Println("⏳ First build may take a few minutes...")

	return sh.RunV("docker", "build",
		"-f", "Dockerfile.dev",
		"-t", devDockerImage,
		".")
}

// Shell opens an interactive shell in the development Docker container
func (Docker) Shell() error {
	fmt.Println("🐳 Opening development Docker shell...")

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if dev image exists
	if err := sh.RunV("docker", "image", "inspect", devDockerImage); err != nil {
		fmt.Printf("❌ Development image '%s' not found\n", devDockerImage)
		fmt.Println("💡 Run 'mage docker:buildDev' first to create the development image")
		return fmt.Errorf("development image not found")
	}

	return sh.RunV("docker", "run", "--rm", "-it",
		"-v", pwd+":/workspace",
		"-w", "/workspace",
		devDockerImage,
		"/bin/bash")
}

// CIShell opens an interactive shell in the CI Docker container (for debugging CI issues)
func (Docker) CIShell() error {
	fmt.Println("🐳 Opening CI Docker shell for debugging...")

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	return sh.RunV("docker", "run", "--rm", "-it",
		"-v", pwd+":/workspace",
		"-w", "/workspace",
		dockerImage,
		"/bin/bash")
}

// CleanDev removes the local development Docker image
func (Docker) CleanDev() error {
	fmt.Println("🐳 Cleaning up development Docker image...")

	// Check if dev image exists
	if err := sh.RunV("docker", "image", "inspect", devDockerImage); err != nil {
		fmt.Printf("💡 Development image '%s' does not exist, nothing to clean\n", devDockerImage)
		return nil
	}

	if err := sh.RunV("docker", "rmi", devDockerImage); err != nil {
		return fmt.Errorf("failed to remove development image: %w", err)
	}

	fmt.Printf("✅ Development image '%s' removed successfully\n", devDockerImage)
	return nil
}

// RebuildDev rebuilds the local development Docker image (useful after go.mod changes)
func (Docker) RebuildDev() error {
	fmt.Println("🐳 Rebuilding development Docker image...")

	// Clean existing image first
	if err := sh.RunV("docker", "image", "inspect", devDockerImage); err == nil {
		fmt.Println("🧹 Removing existing development image...")
		if err := sh.RunV("docker", "rmi", devDockerImage); err != nil {
			fmt.Printf("⚠️  Warning: Failed to remove existing image: %v\n", err)
		}
	}

	// Build new image
	return (Docker{}).BuildDev()
}

// Release builds a release version in Docker with proper versioning
func (Docker) Release(version string) error {
	if version == "" {
		return fmt.Errorf("version is required (e.g., mage docker:release v1.0.0)")
	}

	fmt.Printf("🐳 Building release %s in Docker...\n", version)

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	return sh.RunV("docker", "run", "--rm",
		"-v", pwd+":/workspace",
		"-w", "/workspace",
		dockerImage,
		"mage", "release", version)
}

// Package builds a .deb package using the Ubuntu 20 Docker image
// Usage: mage docker:package v1.0.0 [ubuntu2004|ubuntu2204|ubuntu2404]
func (Docker) Package(version string, ubuntuVersion string) error {
	if version == "" {
		return fmt.Errorf("version is required (e.g., mage docker:package v1.0.0 [ubuntu2004|ubuntu2204|ubuntu2404])")
	}

	// Default to ubuntu2004 if not specified
	targetUbuntu := ubuntuVersion
	if targetUbuntu == "" {
		targetUbuntu = "ubuntu2004"
	}

	// Map Ubuntu version to Docker image
	dockerImageMap := map[string]string{
		"ubuntu2004": "dukeofubuntu/janus-ci:latest-ubuntu20",
		"ubuntu2204": "dukeofubuntu/janus-ci:latest-ubuntu22",
		"ubuntu2404": "dukeofubuntu/janus-ci:latest-ubuntu24",
	}

	targetImage, ok := dockerImageMap[targetUbuntu]
	if !ok {
		return fmt.Errorf("invalid Ubuntu version: %s (must be ubuntu2004, ubuntu2204, or ubuntu2404)", targetUbuntu)
	}

	fmt.Printf("🐳 Building %s package for version %s in Docker...\n", targetUbuntu, version)
	fmt.Printf("📦 Using Docker image: %s\n", targetImage)

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	return sh.RunV("docker", "run", "--rm",
		"-v", pwd+":/workspace",
		"-w", "/workspace",
		targetImage,
		"mage", "package", version, targetUbuntu)
}

// LicenseBuild builds the license-server binary using the development Docker image
func (Docker) LicenseBuild() error {
	fmt.Println("🐳 Building license-server using development Docker image...")
	return runInDevContainer("mage", "license:build")
}

// LicenseTest runs license-server tests using the development Docker image
func (Docker) LicenseTest() error {
	fmt.Println("🐳 Running license-server tests using development Docker image...")
	return runInDevContainer("mage", "license:test")
}

// LicenseUnit runs license-server unit tests using the development Docker image
func (Docker) LicenseUnit() error {
	fmt.Println("🐳 Running license-server unit tests using development Docker image...")
	return runInDevContainer("go", "test", "-v", "-race", "-tags=unit", "./internal/license/...", "./cmd/license-server/...")
}

// LicenseIntegration runs license-server integration tests using the development Docker image
func (Docker) LicenseIntegration() error {
	fmt.Println("🐳 Running license-server integration tests using development Docker image...")
	fmt.Println("⚠️  Starting PostgreSQL container for integration tests...")

	// Start PostgreSQL using docker compose
	if err := sh.RunV("docker", "compose", "--profile", "dev", "up", "-d", "postgres"); err != nil {
		return fmt.Errorf("failed to start PostgreSQL: %w", err)
	}

	// Wait for PostgreSQL to be healthy
	fmt.Println("⏳ Waiting for PostgreSQL to be ready...")
	if err := sh.RunV("docker", "compose", "exec", "-T", "postgres", "pg_isready", "-U", "janus", "-d", "janus_license"); err != nil {
		return fmt.Errorf("PostgreSQL health check failed: %w", err)
	}

	// Get PostgreSQL container network name
	pgNetwork := "janus_default" // docker-compose creates this network by default
	dbURL := "postgres://janus:janus_dev_password@postgres:5432/janus_license?sslmode=disable"

	// Run database migrations
	fmt.Println("🔧 Running database migrations...")
	migrateArgs := []string{
		"run", "--rm",
		"--network", pgNetwork,
		"-v", getCurrentDir() + ":/workspace",
		"-w", "/workspace",
		devDockerImage,
		"go", "run", "./cmd/license-server", "migrate", "up",
		"--database-url", dbURL,
	}
	if err := sh.RunV("docker", migrateArgs...); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	// Run integration tests with database URL
	fmt.Println("🧪 Running integration tests...")
	testArgs := []string{
		"run", "--rm",
		"--network", pgNetwork,
		"-v", getCurrentDir() + ":/workspace",
		"-w", "/workspace",
		"-e", "LICENSING_DATABASE_URL=" + dbURL,
		"-e", "LICENSING_ENCRYPTION_KEY=Wp7G9y+RHAu2QYivFK9jOWZKRs+p7J6Y",
		devDockerImage,
		"go", "test", "-v", "-timeout", "60s", "-tags=integration,server",
		"./internal/license/...", "./cmd/license-server/...",
	}
	err := sh.RunV("docker", testArgs...)

	// Stop PostgreSQL after tests
	fmt.Println("🧹 Stopping PostgreSQL container...")
	if stopErr := sh.RunV("docker", "compose", "--profile", "dev", "down"); stopErr != nil {
		fmt.Printf("⚠️  Warning: Failed to stop PostgreSQL: %v\n", stopErr)
	}

	return err
}

// LicenseCheck runs format, lint, and unit tests for license-server
func (Docker) LicenseCheck() error {
	fmt.Println("🐳 Running license-server checks using development Docker image...")
	return runInDevContainer("mage", "license:check")
}

// LicenseCheckAll runs format, lint, unit tests, and build for license-server
func (Docker) LicenseCheckAll() error {
	fmt.Println("🐳 Running complete license-server validation using development Docker image...")
	return runInDevContainer("mage", "license:checkall")
}

// LicenseRelease builds a release version of license-server in Docker
func (Docker) LicenseRelease(version string) error {
	if version == "" {
		return fmt.Errorf("version is required (e.g., mage docker:licenseRelease v1.0.0)")
	}

	fmt.Printf("🐳 Building license-server release %s in Docker...\n", version)

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	return sh.RunV("docker", "run", "--rm",
		"-v", pwd+":/workspace",
		"-w", "/workspace",
		dockerImage,
		"mage", "license:release", version)
}