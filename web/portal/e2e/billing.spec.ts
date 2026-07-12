import { test, expect } from "@playwright/test";
import { resetMock } from "./support/mock";

test.beforeEach(resetMock);

test("request an agreement, review the offer, accept and pay to activate", async ({ page }) => {
  await page.goto("/orgs/1/billing");
  await expect(page.getByRole("heading", { name: /agreements & billing/i })).toBeVisible();

  // Request an agreement; the mock plays "Janus staff" and prices it instantly.
  await page.getByLabel(/^seats$/i).fill("25");
  await page.getByRole("button", { name: /request agreement/i }).click();

  // Review the priced offer (line items from the catalog, never hardcoded).
  await expect(page.getByText(/Pro per-seat \(annual\)/)).toBeVisible();

  await page.getByRole("button", { name: /accept offer/i }).click();

  // Accepted → pay to start the subscription and activate the agreement.
  const pay = page.getByRole("button", { name: /^pay \$/i });
  await expect(pay).toBeVisible();
  await pay.click();

  await expect(page.getByText(/agreement active/i)).toBeVisible();
});
