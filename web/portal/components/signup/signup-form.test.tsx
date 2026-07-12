import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { signupAction } from "@/lib/signup-actions";
import { SignupForm } from "./signup-form";

// Drive the form with a mocked server action so we can assert the pending →
// success / error transitions without a live license server.
vi.mock("@/lib/signup-actions", () => ({
  signupInitialState: { status: "idle" },
  signupAction: vi.fn(),
}));

async function fillAndSubmit() {
  await userEvent.type(screen.getByLabelText(/organization name/i), "Acme Labs");
  await userEvent.type(screen.getByLabelText(/admin email/i), "owner@acme.io");
  await userEvent.click(screen.getByRole("button", { name: /create account/i }));
}

describe("SignupForm", () => {
  it("renders the signup fields", () => {
    render(<SignupForm />);

    expect(screen.getByLabelText(/organization name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/admin email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/your name/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create account/i })).toBeInTheDocument();
  });

  it("swaps to the check-your-email confirmation on success", async () => {
    vi.mocked(signupAction).mockResolvedValue({ status: "success", email: "owner@acme.io" });
    render(<SignupForm />);

    await fillAndSubmit();

    expect(await screen.findByText(/check your email/i)).toBeInTheDocument();
    expect(screen.getByText("owner@acme.io")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /create account/i })).not.toBeInTheDocument();
  });

  it("surfaces an inline error on failure", async () => {
    vi.mocked(signupAction).mockResolvedValue({
      status: "error",
      message: "Too many attempts. Please wait a little while and try again.",
    });
    render(<SignupForm />);

    await fillAndSubmit();

    expect(await screen.findByRole("alert")).toHaveTextContent(/too many attempts/i);
  });
});
