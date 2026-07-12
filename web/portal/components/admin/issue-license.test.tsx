import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { IssueLicense } from "./issue-license";
import type { AgreementSummary, IssuedLicense, OrgSummary } from "@/lib/types";

const orgs: OrgSummary[] = [
  { id: 7, name: "Acme", customer_id: "CUST-ACME" },
  { id: 9, name: "Globex", customer_id: "CUST-GLOBEX" },
];

const agreements: AgreementSummary[] = [
  { id: 100, organization_id: 7, tier: "enterprise", max_seats: 50, start_date: "2026-01-01T00:00:00Z" },
  { id: 200, organization_id: 9, tier: "team", max_seats: 10, start_date: "2026-01-01T00:00:00Z" },
];

function setup() {
  const result: IssuedLicense = {
    token: "eyJhbGciOi.JWT.sig",
    expires_at: "2027-01-01T00:00:00Z",
  };
  const issue = vi.fn().mockResolvedValue(result);
  render(<IssueLicense orgs={orgs} agreements={agreements} issue={issue} />);

  return issue;
}

describe("IssueLicense", () => {
  it("shows how to generate the signing public key", () => {
    setup();
    expect(screen.getByText(/how does the user generate this key/i)).toBeInTheDocument();
    expect(screen.getByText(/openssl genpkey -algorithm RSA/)).toBeInTheDocument();
    // explicit about the required SPKI format
    expect(screen.getByText(/SPKI/)).toBeInTheDocument();
  });

  it("scopes the agreement picker to the selected org", async () => {
    const user = userEvent.setup();
    setup();

    // default org is Acme (#7) → only its agreement #100 is offered
    expect(screen.getByRole("option", { name: /#100/ })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: /#200/ })).not.toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText(/organization/i), "9");
    expect(screen.getByRole("option", { name: /#200/ })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: /#100/ })).not.toBeInTheDocument();
  });

  it("issues a license with the chosen agreement, email and duration", async () => {
    const user = userEvent.setup();
    const issue = setup();

    await user.selectOptions(screen.getByLabelText(/agreement/i), "100");
    await user.type(screen.getByLabelText(/user email/i), "dev@acme.com");
    await user.type(screen.getByLabelText(/duration/i), "30");
    await user.click(screen.getByRole("button", { name: /issue license/i }));

    await waitFor(() =>
      expect(issue).toHaveBeenCalledWith({
        agreement_id: 100,
        user_email: "dev@acme.com",
        duration_seconds: 30 * 86400,
        signing_public_key: undefined,
      })
    );
    // the signed JWT is surfaced
    expect(await screen.findByText("eyJhbGciOi.JWT.sig")).toBeInTheDocument();
  });

  it("omits duration when left blank (derives from agreement term)", async () => {
    const user = userEvent.setup();
    const issue = setup();

    await user.selectOptions(screen.getByLabelText(/agreement/i), "100");
    await user.type(screen.getByLabelText(/user email/i), "dev@acme.com");
    await user.click(screen.getByRole("button", { name: /issue license/i }));

    await waitFor(() =>
      expect(issue).toHaveBeenCalledWith(
        expect.objectContaining({ agreement_id: 100, duration_seconds: undefined })
      )
    );
  });

  it("requires an agreement selection", async () => {
    const user = userEvent.setup();
    const issue = setup();

    await user.type(screen.getByLabelText(/user email/i), "dev@acme.com");
    await user.click(screen.getByRole("button", { name: /issue license/i }));

    expect(await screen.findByText(/choose an agreement/i)).toBeInTheDocument();
    expect(issue).not.toHaveBeenCalled();
  });
});
