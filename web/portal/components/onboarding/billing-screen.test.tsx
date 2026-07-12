import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { BillingScreen } from "./billing-screen";
import {
  PROPOSAL_ACCEPTED,
  PROPOSAL_FULFILLED,
  PROPOSAL_PROPOSED,
  PROPOSAL_REQUESTED,
  type AgreementProposal,
} from "@/lib/types";

const proposal = (over: Partial<AgreementProposal>): AgreementProposal => ({
  id: 1,
  organization_id: 3,
  requested_by_user_id: 9,
  status: PROPOSAL_REQUESTED,
  seats: 50,
  license_model: "subscription",
  validity_days: 365,
  total_amount_cents: 0,
  created_at: "2026-02-01T00:00:00Z",
  ...over,
});

const proposed = (over: Partial<AgreementProposal> = {}): AgreementProposal =>
  proposal({
    id: 2,
    status: PROPOSAL_PROPOSED,
    total_amount_cents: 6000000,
    line_items: [
      {
        id: 1,
        proposal_id: 2,
        price_id: 7,
        quantity: 50,
        amount_cents: 6000000,
        price: {
          id: 7,
          nickname: "Pro per-seat (annual)",
          unit_amount_cents: 120000,
          currency: "usd",
          interval: "year",
          kind: "per_seat",
        },
      },
    ],
    ...over,
  });

function setup(proposals: AgreementProposal[]) {
  const actions = {
    request: vi.fn().mockResolvedValue(undefined),
    counter: vi.fn().mockResolvedValue(undefined),
    accept: vi.fn().mockResolvedValue(undefined),
    pay: vi.fn().mockResolvedValue(undefined),
  };
  render(<BillingScreen proposals={proposals} {...actions} />);
  return actions;
}

describe("BillingScreen", () => {
  it("requests a new agreement", async () => {
    const user = userEvent.setup();
    const { request } = setup([]);

    await user.clear(screen.getByLabelText(/seats/i));
    await user.type(screen.getByLabelText(/seats/i), "25");
    await user.click(screen.getByRole("button", { name: /request agreement/i }));

    await waitFor(() =>
      expect(request).toHaveBeenCalledWith(expect.objectContaining({ seats: 25 }))
    );
  });

  it("shows line items and total for a proposed offer", () => {
    setup([proposed()]);
    expect(screen.getByText(/Pro per-seat \(annual\)/)).toBeInTheDocument();
    // appears twice: the single line item's amount and the total row
    expect(screen.getAllByText(/\$60,000\.00/)).toHaveLength(2);
    expect(screen.getByText(/total/i)).toBeInTheDocument();
  });

  it("accepts a proposed offer", async () => {
    const user = userEvent.setup();
    const { accept } = setup([proposed({ id: 2 })]);

    await user.click(screen.getByRole("button", { name: /accept offer/i }));
    await waitFor(() => expect(accept).toHaveBeenCalledWith(2));
  });

  it("counters a proposed offer with revised terms", async () => {
    const user = userEvent.setup();
    const { counter } = setup([proposed({ id: 2, seats: 50 })]);

    await user.click(screen.getByRole("button", { name: /counter/i }));
    const seats = screen.getByLabelText(/seats/i);
    await user.clear(seats);
    await user.type(seats, "40");
    await user.click(screen.getByRole("button", { name: /send counter/i }));

    await waitFor(() =>
      expect(counter).toHaveBeenCalledWith(2, expect.objectContaining({ seats: 40 }))
    );
  });

  it("pays an accepted offer", async () => {
    const user = userEvent.setup();
    const { pay } = setup([proposed({ id: 3, status: PROPOSAL_ACCEPTED })]);

    await user.click(screen.getByRole("button", { name: /pay/i }));
    await waitFor(() => expect(pay).toHaveBeenCalledWith(3));
  });

  it("shows an active state once fulfilled", () => {
    setup([proposed({ id: 4, status: PROPOSAL_FULFILLED })]);
    expect(screen.getByText(/agreement active/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /pay|accept/i })).not.toBeInTheDocument();
  });

  it("notes when a request is awaiting an offer", () => {
    setup([proposal({ id: 5, status: PROPOSAL_REQUESTED })]);
    expect(screen.getByText(/awaiting (an )?offer/i)).toBeInTheDocument();
  });
});
