import { test, expect } from "@playwright/test";
import { enableStaff, resetMock } from "./support/mock";

test.beforeEach(resetMock);

test("staff issue a license and get the signed JWT", async ({ page }) => {
  await enableStaff();

  await page.goto("/admin/licenses");
  await expect(page.getByRole("heading", { name: /issue license/i })).toBeVisible();

  // org defaults to Acme; pick its agreement (value = agreement id), fill, issue.
  await page.getByLabel(/agreement/i).selectOption("1");
  await page.getByLabel(/user email/i).fill("dev@acme.test");
  await page.getByRole("button", { name: /issue license/i }).click();

  await expect(page.getByText(/license issued/i)).toBeVisible();
  await expect(page.getByText(/license jwt/i).first()).toBeVisible();
  await expect(page.getByText(/^eyJ\.e2e-issued-/)).toBeVisible();
});

test("issuing without an agreement is blocked client-side", async ({ page }) => {
  await enableStaff();

  await page.goto("/admin/licenses");
  await page.getByLabel(/user email/i).fill("dev@acme.test");
  await page.getByRole("button", { name: /issue license/i }).click();

  await expect(page.getByText(/choose an agreement/i)).toBeVisible();
});
