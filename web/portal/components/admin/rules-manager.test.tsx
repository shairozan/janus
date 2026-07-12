import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { RulesManager } from "./rules-manager";
import { RULE_EMAIL_DOMAIN, RULE_SEAT_THRESHOLD, type AutoAcceptanceRule } from "@/lib/types";

const rule = (over: Partial<AutoAcceptanceRule>): AutoAcceptanceRule => ({
  id: 1,
  organization_id: 3,
  rule_type: RULE_EMAIL_DOMAIN,
  match_domain: "knomix.io",
  enabled: true,
  created_by_user_id: 9,
  created_at: "2026-01-01T00:00:00Z",
  ...over,
});

function setup(rules: AutoAcceptanceRule[]) {
  const actions = {
    create: vi.fn().mockResolvedValue(undefined),
    setEnabled: vi.fn().mockResolvedValue(undefined),
    remove: vi.fn().mockResolvedValue(undefined),
  };
  render(<RulesManager rules={rules} {...actions} />);
  return actions;
}

describe("RulesManager", () => {
  it("lists existing rules with their match condition", () => {
    setup([
      rule({ id: 1, rule_type: RULE_EMAIL_DOMAIN, match_domain: "acme.com" }),
      rule({ id: 2, rule_type: RULE_SEAT_THRESHOLD, match_domain: null, max_auto_seats: 5 }),
    ]);
    expect(screen.getByText(/acme\.com/)).toBeInTheDocument();
    expect(screen.getByText(/up to 5/i)).toBeInTheDocument();
  });

  it("creates an email_domain rule", async () => {
    const user = userEvent.setup();
    const { create } = setup([]);

    await user.type(screen.getByLabelText(/email domain/i), "newco.com");
    await user.click(screen.getByRole("button", { name: /add rule/i }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({ rule_type: RULE_EMAIL_DOMAIN, match_domain: "newco.com" })
    );
  });

  it("creates a seat_threshold rule when that type is selected", async () => {
    const user = userEvent.setup();
    const { create } = setup([]);

    await user.selectOptions(screen.getByLabelText(/rule type/i), RULE_SEAT_THRESHOLD);
    await user.type(screen.getByLabelText(/max auto-approved seats/i), "8");
    await user.click(screen.getByRole("button", { name: /add rule/i }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({ rule_type: RULE_SEAT_THRESHOLD, max_auto_seats: 8 })
    );
  });

  it("toggles a rule's enabled state", async () => {
    const user = userEvent.setup();
    const { setEnabled } = setup([rule({ id: 7, enabled: true })]);

    await user.click(screen.getByRole("button", { name: /disable/i }));
    await waitFor(() => expect(setEnabled).toHaveBeenCalledWith(7, false));
  });

  it("deletes a rule after confirming", async () => {
    const user = userEvent.setup();
    const { remove } = setup([rule({ id: 8 })]);

    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    expect(remove).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: /^confirm$/i }));
    await waitFor(() => expect(remove).toHaveBeenCalledWith(8));
  });
});
