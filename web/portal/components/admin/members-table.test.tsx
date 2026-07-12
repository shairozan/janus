import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { MembersTable } from "./members-table";
import { ROLE_ADMIN, ROLE_MEMBER, type OrgUser } from "@/lib/types";

const member = (over: Partial<OrgUser>): OrgUser => ({
  id: 1,
  organization_id: 3,
  cognito_sub: "s",
  email: "x@x.com",
  role: ROLE_MEMBER,
  source: "sso",
  created_at: "2026-01-01T00:00:00Z",
  ...over,
});

function setup(members: OrgUser[], currentUserId = 99) {
  const actions = { promote: vi.fn().mockResolvedValue(undefined), demote: vi.fn().mockResolvedValue(undefined), offboard: vi.fn().mockResolvedValue(undefined) };
  render(<MembersTable members={members} currentUserId={currentUserId} {...actions} />);
  return actions;
}

describe("MembersTable", () => {
  it("promotes a member", async () => {
    const user = userEvent.setup();
    const { promote } = setup([member({ id: 5, email: "m@x.com", role: ROLE_MEMBER })]);

    await user.click(screen.getByRole("button", { name: /^promote$/i }));
    await waitFor(() => expect(promote).toHaveBeenCalledWith(5));
  });

  it("demotes an admin", async () => {
    const user = userEvent.setup();
    const { demote } = setup([member({ id: 6, email: "a@x.com", role: ROLE_ADMIN })]);

    await user.click(screen.getByRole("button", { name: /^demote$/i }));
    await waitFor(() => expect(demote).toHaveBeenCalledWith(6));
  });

  it("offboard requires a confirm step", async () => {
    const user = userEvent.setup();
    const { offboard } = setup([member({ id: 7, email: "leave@x.com" })]);

    await user.click(screen.getByRole("button", { name: /^offboard$/i }));
    expect(offboard).not.toHaveBeenCalled(); // first click only asks to confirm
    expect(screen.getByText(/offboard leave@x.com\?/i)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^confirm$/i }));
    await waitFor(() => expect(offboard).toHaveBeenCalledWith(7));
  });

  it("disables self demote/offboard", () => {
    setup([member({ id: 42, email: "me@x.com", role: ROLE_ADMIN })], 42);
    expect(screen.getByRole("button", { name: /^demote$/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /^offboard$/i })).toBeDisabled();
  });
});
