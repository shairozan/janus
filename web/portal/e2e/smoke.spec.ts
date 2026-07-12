import { test, expect } from "@playwright/test";

// Smoke e2e proving the Playwright toolchain works against the built app. The
// real journeys (signup, key upload, admin flows) land in B4.
test("landing page renders the portal heading", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: /janus management portal/i })).toBeVisible();
});
