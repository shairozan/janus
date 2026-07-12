import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { SeatsManager } from "./seats-manager";
import type { AgreementUsage, SeatPreview } from "@/lib/types";

const agreement = (over: Partial<AgreementUsage>): AgreementUsage => ({
  id: 11,
  organization_id: 3,
  seats: 10,
  used_seats: 4,
  license_model: "named",
  price_per_seat_cents: 5000,
  ...over,
});

const preview: SeatPreview = {
  current_seats: 10,
  add_seats: 2,
  new_seats: 12,
  prorated_charge_cents: 1234,
  recurring_delta_cents: 10000,
};

function setup(agreements: AgreementUsage[], previewResult: SeatPreview = preview) {
  const actions = {
    preview: vi.fn().mockResolvedValue(previewResult),
    add: vi.fn().mockResolvedValue(agreement({ seats: 12, used_seats: 4 })),
  };
  render(<SeatsManager agreements={agreements} {...actions} />);
  return actions;
}

describe("SeatsManager", () => {
  it("shows seat usage per agreement", () => {
    setup([agreement({ id: 11, seats: 10, used_seats: 4 })]);
    expect(screen.getByText(/4 \/ 10 seats/i)).toBeInTheDocument();
  });

  it("fetches a proration preview before confirming", async () => {
    const user = userEvent.setup();
    const { preview: previewFn, add } = setup([agreement({ id: 11 })]);

    await user.type(screen.getByLabelText(/add seats/i), "2");
    await user.click(screen.getByRole("button", { name: /preview/i }));

    await waitFor(() => expect(previewFn).toHaveBeenCalledWith(11, 2));
    expect(await screen.findByText(/\$12\.34/)).toBeInTheDocument(); // prorated charge now
    expect(add).not.toHaveBeenCalled(); // preview does not commit
  });

  it("applies the seat add after preview", async () => {
    const user = userEvent.setup();
    const { add } = setup([agreement({ id: 11 })]);

    await user.type(screen.getByLabelText(/add seats/i), "2");
    await user.click(screen.getByRole("button", { name: /preview/i }));
    await screen.findByText(/\$12\.34/);

    await user.click(screen.getByRole("button", { name: /confirm & add/i }));
    await waitFor(() => expect(add).toHaveBeenCalledWith(11, 2));
  });

  it("warns when an agreement is full", () => {
    setup([agreement({ id: 11, seats: 5, used_seats: 5 })]);
    expect(screen.getByText(/no seats available/i)).toBeInTheDocument();
  });
});
