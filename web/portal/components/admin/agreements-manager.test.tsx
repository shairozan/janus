import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { AgreementsManager } from "./agreements-manager";
import type { AgreementSummary, LicenseTier, OrgSummary } from "@/lib/types";

vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh: vi.fn() }) }));

const orgs: OrgSummary[] = [{ id: 7, name: "Pharmalytica", customer_id: "CUST-INTERNAL-001" }];

const licenses: LicenseTier[] = [
  { id: 1, name: "Professional License", tier: "professional", features: ["basic", "local", "grid"], msrp_cents: 5000 },
  { id: 2, name: "Enterprise License", tier: "enterprise", features: ["basic", "local", "grid", "audit"], msrp_cents: 10000 },
];

function setup(agreements: AgreementSummary[] = []) {
  const create = vi.fn().mockResolvedValue({
    id: 42,
    organization_id: 7,
    tier: "professional",
    max_seats: 5,
    start_date: "2026-06-08T00:00:00Z",
  } satisfies AgreementSummary);
  render(<AgreementsManager agreements={agreements} orgs={orgs} licenses={licenses} create={create} />);

  return create;
}

describe("AgreementsManager", () => {
  it("creates an agreement carrying the selected tier's features and a default term", async () => {
    const user = userEvent.setup();
    const create = setup();

    await user.click(screen.getByRole("button", { name: /create agreement/i }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          organization_id: 7,
          tier: "professional",
          max_seats: 5,
          features: ["basic", "local", "grid"],
          cost_per_month: undefined,
          start_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
          end_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
        })
      )
    );
    expect(await screen.findByText(/agreement #42/i)).toBeInTheDocument();
  });

  it("switches tier and sends the new tier's features", async () => {
    const user = userEvent.setup();
    const create = setup();

    await user.selectOptions(screen.getByLabelText(/tier/i), "enterprise");
    expect(screen.getByText(/audit/)).toBeInTheDocument(); // features preview updates
    await user.click(screen.getByRole("button", { name: /create agreement/i }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({ tier: "enterprise", features: ["basic", "local", "grid", "audit"] })
      )
    );
  });

  it("sends cost_per_month when provided", async () => {
    const user = userEvent.setup();
    const create = setup();

    await user.type(screen.getByLabelText(/cost \/ month/i), "1200");
    await user.click(screen.getByRole("button", { name: /create agreement/i }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith(expect.objectContaining({ cost_per_month: 1200 }))
    );
  });

  it("lists existing agreements", () => {
    setup([
      { id: 9, organization_id: 7, tier: "enterprise", max_seats: 50, start_date: "2026-01-01T00:00:00Z" },
    ]);
    expect(screen.getByText("#9")).toBeInTheDocument();
    expect(screen.getByText("enterprise")).toBeInTheDocument();
    expect(screen.getByText(/50 seats/i)).toBeInTheDocument();
  });

  it("blocks submit when no tier is defined", async () => {
    const user = userEvent.setup();
    const create = vi.fn();
    render(<AgreementsManager agreements={[]} orgs={orgs} licenses={[]} create={create} />);

    await user.click(screen.getByRole("button", { name: /create agreement/i }));

    expect(await screen.findByText(/select a tier/i)).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });
});
