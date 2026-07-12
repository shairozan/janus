import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { SigningKeys } from "./signing-keys";
import type { OrgSummary, RotateKeyResult, SigningKey } from "@/lib/types";

// The component calls router.refresh() after a rotation to re-pull the key list.
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh: vi.fn() }) }));

const orgs: OrgSummary[] = [
  { id: 7, name: "Acme", customer_id: "CUST-ACME" },
  { id: 9, name: "Globex", customer_id: "CUST-GLOBEX" },
];

const key = (over: Partial<SigningKey>): SigningKey => ({
  key_id: "key-1",
  organization_id: null,
  public_key_pem: "-----BEGIN PUBLIC KEY-----\nAAA\n-----END PUBLIC KEY-----",
  active: true,
  created_at: "2026-01-01T00:00:00Z",
  expires_at: "2027-01-01T00:00:00Z",
  ...over,
});

function setup(keys: SigningKey[]) {
  const result: RotateKeyResult = {
    new_key_id: "key-555",
    old_key_id: "key-1",
    expires_at: "2027-06-01T00:00:00Z",
    public_key: "-----BEGIN PUBLIC KEY-----\nNEW\n-----END PUBLIC KEY-----",
  };
  const rotate = vi.fn().mockResolvedValue(result);
  render(<SigningKeys keys={keys} orgs={orgs} rotate={rotate} />);

  return rotate;
}

describe("SigningKeys", () => {
  it("lists keys with their status and scope", () => {
    setup([
      key({ key_id: "key-active", active: true, organization_id: null }),
      key({ key_id: "key-org", active: false, organization_id: 7 }),
      key({ key_id: "key-revoked", revoked_at: "2026-02-02T00:00:00Z", organization_id: 9 }),
    ]);

    expect(screen.getByText("key-active")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByText("Revoked")).toBeInTheDocument();
    expect(screen.getByText(/Master \(global\)/)).toBeInTheDocument();
    expect(screen.getByText("Acme")).toBeInTheDocument();
  });

  it("shows an empty state with no keys", () => {
    setup([]);
    expect(screen.getByText(/no signing keys yet/i)).toBeInTheDocument();
  });

  it("rotates the master key with the default validity", async () => {
    const user = userEvent.setup();
    const rotate = setup([]);

    await user.click(screen.getByRole("button", { name: /rotate key/i }));

    await waitFor(() =>
      expect(rotate).toHaveBeenCalledWith({ organization_id: undefined, expires_in_days: 365 })
    );
    // surfaces the new key + PEM
    expect(await screen.findByText("key-555")).toBeInTheDocument();
    expect(screen.getByText(/public key \(pem\)/i)).toBeInTheDocument();
  });

  it("rotates a chosen org's key", async () => {
    const user = userEvent.setup();
    const rotate = setup([]);

    await user.selectOptions(screen.getByLabelText(/scope/i), "9");
    await user.click(screen.getByRole("button", { name: /rotate key/i }));

    await waitFor(() =>
      expect(rotate).toHaveBeenCalledWith({ organization_id: 9, expires_in_days: 365 })
    );
  });

  it("surfaces a rotation error", async () => {
    const user = userEvent.setup();
    const rotate = vi.fn().mockRejectedValue(new Error("no permission"));
    render(<SigningKeys keys={[]} orgs={orgs} rotate={rotate} />);

    await user.click(screen.getByRole("button", { name: /rotate key/i }));

    expect(await screen.findByText(/no permission/i)).toBeInTheDocument();
  });

  it("enables the embed download when global keys exist", () => {
    setup([key({ key_id: "key-active", organization_id: null })]);
    expect(
      screen.getByRole("button", { name: /download \.license_public_key\.pem/i })
    ).toBeEnabled();
  });

  it("disables the embed download with no global keys (only org-scoped)", () => {
    setup([key({ key_id: "key-org", organization_id: 7 })]);
    expect(
      screen.getByRole("button", { name: /download \.license_public_key\.pem/i })
    ).toBeDisabled();
  });
});
