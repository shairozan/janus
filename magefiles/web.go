//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Web contains targets for the Next.js management portal (web/portal). They shell
// out to pnpm, keeping the Go toolchain entirely separate — the root Go targets
// (Check/CheckAll) are intentionally NOT coupled to Node so they keep working on
// machines without it.
type Web mg.Namespace

const portalDir = "web/portal"

// pnpm runs a pnpm command inside the portal project (web/portal) via `-C`.
func pnpm(args ...string) error {
	return sh.RunV("pnpm", append([]string{"-C", portalDir}, args...)...)
}

// Install installs the portal's dependencies (frozen to the lockfile).
func (Web) Install() error {
	fmt.Println("📦 Installing portal dependencies...")
	return pnpm("install", "--frozen-lockfile")
}

// Lint runs ESLint over the portal.
func (Web) Lint() error {
	fmt.Println("🔎 Linting portal...")
	return pnpm("lint")
}

// Typecheck runs the TypeScript compiler in no-emit mode.
func (Web) Typecheck() error {
	fmt.Println("🧠 Type-checking portal...")
	return pnpm("typecheck")
}

// Test runs the Vitest unit/component suite.
func (Web) Test() error {
	fmt.Println("🧪 Testing portal...")
	return pnpm("test")
}

// Build builds the production (standalone) Next bundle.
func (Web) Build() error {
	fmt.Println("🔨 Building portal...")
	return pnpm("build")
}

// E2e runs the hermetic Playwright suite in the official Playwright container
// (browsers aren't installable on every host OS — the container sidesteps that).
// Mirrors the portal-e2e CI workflow.
func (Web) E2e() error {
	fmt.Println("🎭 Running portal e2e (containerized)...")
	return sh.RunV("docker", "compose", "-f", portalDir+"/docker-compose.e2e.yml", "run", "--rm", "e2e")
}

// Check runs lint, typecheck, and unit tests (fast local validation).
func (Web) Check() error {
	mg.Deps(Web.Lint, Web.Typecheck, Web.Test)
	fmt.Println("✅ Portal checks passed!")
	return nil
}

// CheckAll runs lint, typecheck, unit tests, and the production build. The e2e
// suite (Web.E2e) is heavyweight/containerized and runs separately.
func (Web) CheckAll() error {
	mg.Deps(Web.Lint, Web.Typecheck, Web.Test, Web.Build)
	fmt.Println("✅ Complete portal validation passed!")
	return nil
}
