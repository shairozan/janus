import { test, expect } from "@playwright/test";
import { PUBLIC_KEY_PEM, SECOND_KEY_PEM, resetMock } from "./support/mock";

test.beforeEach(resetMock);

test("first key upload issues a downloadable license", async ({ page }) => {
  await page.goto("/me");
  await expect(page.getByRole("heading", { name: /your account/i })).toBeVisible();
  await expect(page.getByText(/no active license yet/i)).toBeVisible();

  await page.getByLabel(/public key \(pem\)/i).fill(PUBLIC_KEY_PEM);
  await page.getByLabel(/label \(optional\)/i).fill("work laptop");
  await page.getByRole("button", { name: /add key/i }).click();

  // After the server action revalidates /me, the active key + license appear.
  await expect(page.getByText(/active key/i)).toBeVisible();
  await expect(page.getByRole("button", { name: /download license/i })).toBeVisible();
});

test("replacing the key requires the caveat ack and reissues the license", async ({ page }) => {
  await page.goto("/me");

  // Seed an active key (first upload).
  await page.getByLabel(/public key \(pem\)/i).fill(PUBLIC_KEY_PEM);
  await page.getByRole("button", { name: /add key/i }).click();
  await expect(page.getByText(/active key/i)).toBeVisible();

  // A second upload is a REPLACEMENT — submit is gated on the acknowledgement.
  await page.getByLabel(/new public key \(pem\)/i).fill(SECOND_KEY_PEM);
  const submit = page.getByRole("button", { name: /replace key/i });
  await expect(submit).toBeDisabled();
  await expect(page.getByText(/replacing your key reissues your license/i)).toBeVisible();

  await page.getByLabel(/i understand the consequences/i).check();
  await expect(submit).toBeEnabled();
  await submit.click();

  // The prior key is now revoked history; a fresh license is still downloadable.
  await expect(page.getByText(/previous keys/i)).toBeVisible();
  await expect(page.getByRole("button", { name: /download license/i })).toBeVisible();
});
