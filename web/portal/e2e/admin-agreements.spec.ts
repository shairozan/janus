import { test, expect } from "@playwright/test";
import { enableStaff, resetMock } from "./support/mock";

test.beforeEach(resetMock);

test("staff define an agreement directly and see it listed", async ({ page }) => {
  await enableStaff();
  await page.goto("/admin/agreements");
  await expect(page.getByRole("heading", { name: /agreements/i, level: 1 })).toBeVisible();

  // defaults: org Acme, tier professional, seats 5, term auto-filled
  await page.getByRole("button", { name: /create agreement/i }).click();

  await expect(page.getByText(/created/i)).toBeVisible();
  // the new agreement (tier "professional") shows in the list alongside the seed
  await expect(page.getByRole("list").getByText("professional")).toBeVisible();
});

test("a staff-created agreement is issuable from Issue License", async ({ page }) => {
  await enableStaff();

  await page.goto("/admin/agreements");
  await page.getByRole("button", { name: /create agreement/i }).click();
  await expect(page.getByText(/created/i)).toBeVisible();

  // The created agreement (id 1001 — first nextID after the 1000 seed) is now
  // selectable for issuance under the same org.
  await page.goto("/admin/licenses");
  await page.getByLabel(/agreement/i).selectOption("1001");
  await page.getByLabel(/user email/i).fill("dev@acme.test");
  await page.getByRole("button", { name: /issue license/i }).click();

  await expect(page.getByText(/license issued/i)).toBeVisible();
  await expect(page.getByText(/^eyJ\.e2e-issued-/)).toBeVisible();
});
