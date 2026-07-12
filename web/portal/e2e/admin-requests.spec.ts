import { test, expect } from "@playwright/test";
import { resetMock } from "./support/mock";

test.beforeEach(resetMock);

test("admin approves and rejects pending license requests", async ({ page }) => {
  await page.goto("/orgs/1/requests");
  await expect(page.getByRole("heading", { name: /license requests/i })).toBeVisible();
  await expect(page.getByText(/pending \(2\)/i)).toBeVisible();

  // Approve the first pending request → it drops into history as "approved".
  await page.getByRole("button", { name: /^approve$/i }).first().click();
  await expect(page.getByText("approved")).toBeVisible();
  await expect(page.getByText(/pending \(1\)/i)).toBeVisible();

  // Reject the remaining one with a reason.
  await page.getByRole("button", { name: /^reject$/i }).first().click();
  await page.getByLabel(/rejection reason/i).fill("not this cycle");
  await page.getByRole("button", { name: /confirm reject/i }).click();
  await expect(page.getByText("rejected")).toBeVisible();
  await expect(page.getByText(/pending \(0\)/i)).toBeVisible();
});
