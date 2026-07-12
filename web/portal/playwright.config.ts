import { defineConfig, devices } from "@playwright/test";
import { STORAGE_STATE } from "./e2e/global-setup";

// Hermetic e2e: Playwright drives the real portal UI in a headless browser while
// the license-server API is served by an in-memory mock (e2e/support/mock-server
// .mjs) and the Cognito session is injected as a storageState cookie (e2e/global
// -setup.ts). No database, no AWS/Stripe/Mailgun — matching the repo's IQ/OQ
// "mock externals" stance. Browser provided by the Playwright container in
// docker-compose.e2e.yml (and by `playwright install` on CI's Ubuntu runner).

const CI = !!process.env.CI;
const MOCK_PORT = process.env.MOCK_PORT ?? "4319";

// A fixed test secret shared by global-setup (mints the cookie) and the portal
// webServer (decodes it). Never used in production. Override via AUTH_SECRET.
const AUTH_SECRET = process.env.AUTH_SECRET ?? "e2e-only-secret-do-not-use-in-prod-00000=";
process.env.AUTH_SECRET = AUTH_SECRET;

const portalEnv = {
  AUTH_SECRET,
  AUTH_URL: "http://localhost:3000",
  AUTH_TRUST_HOST: "true",
  LICENSE_SERVER_URL: `http://localhost:${MOCK_PORT}`,
  // The Cognito provider is configured but never exercised (the session is
  // injected); dummy non-empty values keep Auth.js from erroring at startup.
  AUTH_COGNITO_ID: "e2e-client",
  AUTH_COGNITO_SECRET: "e2e-client-secret",
  AUTH_COGNITO_ISSUER: "https://cognito-idp.us-east-2.amazonaws.com/us-east-2_e2e",
};

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false, // the mock holds shared state; reset per-test under one worker
  workers: 1,
  forbidOnly: CI,
  retries: CI ? 1 : 0,
  reporter: CI ? [["github"], ["html", { open: "never" }]] : "html",
  globalSetup: "./e2e/global-setup.ts",
  use: {
    baseURL: "http://localhost:3000",
    storageState: STORAGE_STATE,
    trace: "on-first-retry",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      command: "node e2e/support/mock-server.mjs",
      url: `http://localhost:${MOCK_PORT}/__health`,
      reuseExistingServer: !CI,
      env: { MOCK_PORT },
    },
    {
      command: "pnpm build && pnpm start",
      url: "http://localhost:3000",
      reuseExistingServer: !CI,
      timeout: 180_000,
      env: portalEnv,
    },
  ],
});
