import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { ProposalForm } from "./proposal-form";

describe("ProposalForm", () => {
  it("submits the parsed terms", async () => {
    const user = userEvent.setup();
    const submit = vi.fn().mockResolvedValue(undefined);
    render(<ProposalForm submit={submit} submitLabel="Request agreement" />);

    await user.clear(screen.getByLabelText(/seats/i));
    await user.type(screen.getByLabelText(/seats/i), "50");
    await user.selectOptions(screen.getByLabelText(/license model/i), "concurrent");
    await user.type(screen.getByLabelText(/note/i), "academic pricing?");
    await user.click(screen.getByRole("button", { name: /request agreement/i }));

    await waitFor(() =>
      expect(submit).toHaveBeenCalledWith(
        expect.objectContaining({
          seats: 50,
          license_model: "concurrent",
          message: "academic pricing?",
        })
      )
    );
  });

  it("prefills from defaults (used when countering)", () => {
    render(
      <ProposalForm
        submit={vi.fn()}
        submitLabel="Send counter"
        defaults={{ seats: 75, license_model: "named", tier: "enterprise" }}
      />
    );
    expect(screen.getByLabelText(/seats/i)).toHaveValue(75);
    expect(screen.getByLabelText(/license model/i)).toHaveValue("named");
  });

  it("requires a positive seat count", async () => {
    const user = userEvent.setup();
    const submit = vi.fn();
    render(<ProposalForm submit={submit} submitLabel="Request agreement" />);

    await user.clear(screen.getByLabelText(/seats/i));
    await user.type(screen.getByLabelText(/seats/i), "0");
    expect(screen.getByRole("button", { name: /request agreement/i })).toBeDisabled();
  });
});
