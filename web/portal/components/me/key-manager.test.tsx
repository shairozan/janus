import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { KeyManager } from "./key-manager";
import type { PublicKey } from "@/lib/types";

const VALID_PEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0Z
-----END PUBLIC KEY-----`;

const activeKey: PublicKey = {
  id: 1,
  org_user_id: 7,
  fingerprint: "SHA256:abc",
  title: "laptop",
  created_at: "2026-01-01T00:00:00Z",
};

function submitButton() {
  return screen.getByRole("button", { name: /add key|replace key|saving/i });
}

describe("KeyManager — first upload (no active key)", () => {
  it("enables submit only once a valid PEM is entered, then calls onReplace", async () => {
    const user = userEvent.setup();
    const onReplace = vi.fn().mockResolvedValue({});
    render(<KeyManager active={null} history={[]} onReplace={onReplace} />);

    expect(submitButton()).toBeDisabled();

    await user.type(screen.getByLabelText(/public key \(pem\)/i), "garbage");
    expect(submitButton()).toBeDisabled();
    expect(screen.getByText(/doesn.t look like a public key/i)).toBeInTheDocument();

    await user.clear(screen.getByLabelText(/public key \(pem\)/i));
    await user.type(screen.getByLabelText(/public key \(pem\)/i), VALID_PEM);
    expect(submitButton()).toBeEnabled();

    await user.click(submitButton());
    await waitFor(() =>
      expect(onReplace).toHaveBeenCalledWith(expect.objectContaining({ publicKeyPem: VALID_PEM }))
    );
  });

  it("shows no rotation caveat on a first upload", () => {
    render(<KeyManager active={null} history={[]} onReplace={vi.fn()} />);
    expect(screen.queryByText(/reissues your license/i)).not.toBeInTheDocument();
  });
});

describe("KeyManager — replacement (active key exists)", () => {
  it("requires the explicit acknowledgement before submit is enabled", async () => {
    const user = userEvent.setup();
    const onReplace = vi.fn().mockResolvedValue({});
    render(<KeyManager active={activeKey} history={[]} onReplace={onReplace} />);

    // caveat is shown
    expect(screen.getByText(/reissues your license/i)).toBeInTheDocument();

    await user.type(screen.getByLabelText(/new public key \(pem\)/i), VALID_PEM);
    // valid PEM but not acknowledged → still disabled
    expect(submitButton()).toBeDisabled();

    await user.click(screen.getByLabelText(/i understand the consequences/i));
    expect(submitButton()).toBeEnabled();

    await user.click(submitButton());
    await waitFor(() => expect(onReplace).toHaveBeenCalledTimes(1));
  });

  it("surfaces an error from onReplace", async () => {
    const user = userEvent.setup();
    const onReplace = vi.fn().mockRejectedValue(new Error("no seats available"));
    render(<KeyManager active={activeKey} history={[]} onReplace={onReplace} />);

    await user.type(screen.getByLabelText(/new public key \(pem\)/i), VALID_PEM);
    await user.click(screen.getByLabelText(/i understand the consequences/i));
    await user.click(submitButton());

    await waitFor(() => expect(screen.getByText(/no seats available/i)).toBeInTheDocument());
  });
});
