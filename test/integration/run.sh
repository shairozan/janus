#!/usr/bin/env bash
# Runs the non-hermetic integration E2E stack and exits with the Playwright
# suite's code. Used by both local runs and CI
# (.github/workflows/portal-integration-e2e.yml).
#
# We start the stack detached and `docker wait` the playwright container rather
# than `up --abort-on-container-exit`, because the one-shot jobs (oidc-ready,
# seed) exit 0 early and would otherwise abort the whole stack before the test.
set -euo pipefail

cd "$(dirname "$0")/../.." # repo root
COMPOSE=(docker compose -f docker-compose.integration.yml)

cleanup() { "${COMPOSE[@]}" down -v >/dev/null 2>&1 || true; }
trap cleanup EXIT

"${COMPOSE[@]}" up -d --build

pw="$("${COMPOSE[@]}" ps -q playwright)"
status="$(docker wait "$pw")"

echo "=== playwright logs ==="
"${COMPOSE[@]}" logs playwright

echo "=== playwright exit: ${status} ==="
exit "${status}"
