import { test, expect } from "@playwright/test";

// Must match the cognito_sub seeded in test/integration/seed.sql — the subject we
// submit to the mock OIDC server becomes the token's `sub`, which ResolveOrgUser
// matches against.
const SUBJECT = "integration-user";

// The full real chain: portal → mock OIDC → portal callback → /me, with the
// license-server actually validating the token and resolving the org user, and
// the real /me handlers returning real (empty) collections. This is everything
// the hermetic suite mocks away.
test("real login reaches /me and renders the seeded profile", async ({ page }) => {
  await page.goto("/me");

  // Unauthenticated → middleware bounces to /login. Click through to the IdP.
  await page.getByRole("button", { name: /^sign in$/i }).click();

  // Mock OIDC interactive login — submit our seeded subject plus the Cognito-
  // shaped claims the license-server validates (`client_id`, `token_use`,
  // groups). The mock's interactive auth-code flow takes these from the form's
  // `claims` field; the tokenCallbacks config alone doesn't set `client_id` here.
  await page.locator('input[name="username"]').fill(SUBJECT);
  await page.locator('[name="claims"]').fill(
    JSON.stringify({
      client_id: "integration-client",
      token_use: "access",
      "cognito:groups": ["customer_admin"],
    })
  );
  await page.getByRole("button", { name: /sign.?in/i }).click();

  // Back in the app, authenticated, on /me.
  await expect(page).toHaveURL(/\/me$/);

  // Profile rendered → token validated + ResolveOrgUser found the seeded user.
  // (Email shows in the nav, header, and profile field — assert it's present.)
  await expect(page.getByText("integration@example.test").first()).toBeVisible();
  await expect(page.getByText(/customer admin/i).first()).toBeVisible();

  // No keys/requests yet → the page renders the empty state instead of crashing
  // on a nil-slice→null (the bug the hermetic suite couldn't catch).
  await expect(page.getByText(/no active key yet/i)).toBeVisible();
});
