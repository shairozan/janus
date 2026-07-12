import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ActivityLog } from "./activity-log";
import type { AuditEvent } from "@/lib/types";

const event = (over: Partial<AuditEvent>): AuditEvent => ({
  id: "0190000000007000",
  occurred_at: "2026-06-01T12:00:00Z",
  organization_id: 3,
  actor_org_user_id: 5,
  actor_label: "admin@x.com",
  resource: "license",
  action: "approve",
  resource_id: "42",
  request_method: "POST",
  request_path: "/api/v1/orgs/3/license-requests/42/approve",
  result_status: 200,
  ...over,
});

describe("ActivityLog", () => {
  it("renders each event's actor, resource and action", () => {
    render(
      <ActivityLog
        events={[
          event({ id: "a", actor_label: "alice@x.com", resource: "member", action: "promote" }),
          event({ id: "b", actor_label: "bob@x.com", resource: "seat", action: "add" }),
        ]}
      />
    );

    expect(screen.getByText("alice@x.com")).toBeInTheDocument();
    expect(screen.getByText("bob@x.com")).toBeInTheDocument();
    expect(screen.getByText(/member/)).toBeInTheDocument();
    expect(screen.getByText(/promote/)).toBeInTheDocument();
  });

  it("flags non-2xx results", () => {
    render(<ActivityLog events={[event({ id: "f", result_status: 403 })]} />);
    expect(screen.getByText("403")).toBeInTheDocument();
  });

  it("shows an empty state when there is no activity", () => {
    render(<ActivityLog events={[]} />);
    expect(screen.getByText(/no activity yet/i)).toBeInTheDocument();
  });
});
