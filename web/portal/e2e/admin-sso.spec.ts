import { test, expect } from "@playwright/test";
import { resetMock } from "./support/mock";

test.beforeEach(resetMock);

test("admin sets up SSO and gets a copyable login URL", async ({ page }) => {
  await page.goto("/orgs/1/sso");
  await expect(page.getByRole("heading", { name: /single sign-on/i })).toBeVisible();

  await page.getByLabel(/provider name/i).fill("acme-okta");
  await page.getByLabel(/oidc issuer/i).fill("https://acme.okta.com");
  await page.getByLabel(/client id/i).fill("client-123");
  await page.getByLabel(/client secret/i).fill("s3cr3t");
  await page.getByRole("button", { name: /create sso/i }).click();

  // Create-once → the read-only view, with the IdP deep-link and a support note.
  await expect(page.getByText(/contact support/i)).toBeVisible();
  await expect(page.getByText(/secret stored/i)).toBeVisible();
  await expect(page.getByRole("textbox", { name: /login url/i })).toHaveValue(
    /identity_provider=acme-okta/
  );
});
