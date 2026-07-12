import { readFileSync } from "node:fs";
import { test, expect } from "@playwright/test";
import { enableStaff, resetMock } from "./support/mock";

test.beforeEach(resetMock);

test("non-staff users cannot reach the Admin area", async ({ page }) => {
  // /me reports is_staff:false by default → requireStaff redirects to /me.
  await page.goto("/admin/keys");
  await expect(page).toHaveURL(/\/me$/);
  await expect(page.getByRole("button", { name: /admin/i })).toHaveCount(0);
});

test("staff reach Signing Keys via the Admin dropdown", async ({ page }) => {
  await enableStaff();

  await page.goto("/me");
  await page.getByRole("button", { name: /admin/i }).click();
  await page.getByRole("menuitem", { name: /signing keys/i }).click();

  await expect(page).toHaveURL(/\/admin\/keys$/);
  await expect(page.getByRole("heading", { name: /signing keys/i, level: 1 })).toBeVisible();
  // seeded master key renders in the list
  await expect(page.getByRole("list").getByText("key-seed")).toBeVisible();
});

test("staff rotate a signing key and see the new key + PEM", async ({ page }) => {
  await enableStaff();

  await page.goto("/admin/keys");
  await page.getByRole("button", { name: /rotate key/i }).click();

  await expect(page.getByText(/new key/i)).toBeVisible();
  await expect(page.getByText(/public key \(pem\)/i)).toBeVisible();
  // prior key retired but still listed
  await expect(page.getByRole("list").getByText("key-seed")).toBeVisible();
});

test("staff download the concatenated embed file for the build", async ({ page }) => {
  await enableStaff();
  await page.goto("/admin/keys");

  // Rotate first so there are TWO non-revoked global keys to concatenate.
  await page.getByRole("button", { name: /rotate key/i }).click();
  await expect(page.getByText(/new key/i)).toBeVisible();

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: /download \.license_public_key\.pem/i }).click(),
  ]);

  // Chromium strips the leading dot from dotfile downloads, so the suggested
  // name may be "license_public_key.pem" — the UI tells the user to restore it.
  expect(download.suggestedFilename()).toMatch(/^\.?license_public_key\.pem$/);

  const path = await download.path();
  const content = readFileSync(path, "utf8");
  // both global keys' public PEM blocks are present (seed + rotated)
  const blocks = content.match(/-----BEGIN PUBLIC KEY-----/g) ?? [];
  expect(blocks.length).toBe(2);
  expect(content).toContain("SEED"); // the seeded key's material
  expect(content.endsWith("\n")).toBe(true);
});
