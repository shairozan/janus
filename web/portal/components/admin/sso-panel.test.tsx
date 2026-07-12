import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { SSOPanel } from "./sso-panel";
import type { SSOConfig } from "@/lib/types";

const config = (over: Partial<SSOConfig> = {}): SSOConfig => ({
  id: 1,
  organization_id: 3,
  provider_name: "acme-okta",
  user_pool_id: "us-east-1_abc",
  oidc_issuer: "https://acme.okta.com",
  oidc_client_id: "client-123",
  scopes: "openid email profile",
  attribute_mapping: { email: "email" },
  cognito_status: "active",
  login_url: "https://auth.januspk.com/login?identity_provider=acme-okta",
  has_client_secret: true,
  ...over,
});

describe("SSOPanel", () => {
  it("shows the setup wizard when no config exists", async () => {
    const user = userEvent.setup();
    const setup = vi.fn().mockResolvedValue(config());
    render(<SSOPanel config={null} setup={setup} />);

    await user.type(screen.getByLabelText(/provider name/i), "acme-okta");
    await user.type(screen.getByLabelText(/issuer/i), "https://acme.okta.com");
    await user.type(screen.getByLabelText(/client id/i), "client-123");
    await user.type(screen.getByLabelText(/client secret/i), "shhh");
    await user.click(screen.getByRole("button", { name: /create sso/i }));

    await waitFor(() =>
      expect(setup).toHaveBeenCalledWith(
        expect.objectContaining({
          provider_name: "acme-okta",
          oidc_issuer: "https://acme.okta.com",
          client_id: "client-123",
          client_secret: "shhh",
        })
      )
    );
  });

  it("renders a read-only view when a config exists", () => {
    render(<SSOPanel config={config()} setup={vi.fn()} />);

    expect(screen.getByText("acme-okta")).toBeInTheDocument();
    expect(screen.getByText("client-123")).toBeInTheDocument();
    expect(
      screen.getByDisplayValue("https://auth.januspk.com/login?identity_provider=acme-okta")
    ).toBeInTheDocument();
    // no edit/delete affordances — changes are support-only
    expect(screen.queryByRole("button", { name: /delete|edit|save/i })).not.toBeInTheDocument();
    expect(screen.getByText(/contact support/i)).toBeInTheDocument();
  });

  it("never reveals the client secret, only its presence", () => {
    render(<SSOPanel config={config({ has_client_secret: true })} setup={vi.fn()} />);
    expect(screen.getByText(/secret stored/i)).toBeInTheDocument();
    expect(screen.queryByText("shhh")).not.toBeInTheDocument();
  });
});
