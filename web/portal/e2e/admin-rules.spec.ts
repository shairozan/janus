import { test, expect } from "@playwright/test";
import { resetMock } from "./support/mock";

test.beforeEach(resetMock);

test("admin creates an email-domain auto-acceptance rule", async ({ page }) => {
  await page.goto("/orgs/1/rules");
  await expect(page.getByRole("heading", { name: /auto-acceptance rules/i })).toBeVisible();
  await expect(page.getByText(/no rules yet/i)).toBeVisible();

  await page.getByLabel(/^email domain$/i).fill("acme.test");
  await page.getByRole("button", { name: /add rule/i }).click();

  await expect(page.getByText(/auto-approve @acme\.test/i)).toBeVisible();
  await expect(page.getByText("Enabled")).toBeVisible();
});
