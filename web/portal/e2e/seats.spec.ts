import { test, expect } from "@playwright/test";
import { resetMock } from "./support/mock";

test.beforeEach(resetMock);

test("admin previews proration before adding seats", async ({ page }) => {
  await page.goto("/orgs/1/seats");
  await expect(page.getByRole("heading", { name: /^seats$/i })).toBeVisible();
  await expect(page.getByText(/4 \/ 10 seats/i)).toBeVisible();

  await page.getByLabel(/add seats/i).fill("2");
  await page.getByRole("button", { name: /^preview$/i }).click();

  // The proration preview is shown before anything is charged.
  await expect(page.getByText(/prorated charge now/i)).toBeVisible();
  await page.getByRole("button", { name: /confirm & add/i }).click();

  // After the add + revalidate, the new total is reflected.
  await expect(page.getByText(/4 \/ 12 seats/i)).toBeVisible();
});
