"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ROLE_ADMIN, type OrgUser } from "@/lib/types";

interface MembersTableProps {
  members: OrgUser[];
  currentUserId: number;
  promote: (userID: number) => Promise<void>;
  demote: (userID: number) => Promise<void>;
  offboard: (userID: number) => Promise<void>;
}

export function MembersTable({ members, currentUserId, promote, demote, offboard }: MembersTableProps) {
  const [busy, setBusy] = useState<number | null>(null);
  const [confirming, setConfirming] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function run(userID: number, fn: (id: number) => Promise<void>) {
    setBusy(userID);
    setError(null);
    try {
      await fn(userID);
      setConfirming(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Action failed");
    } finally {
      setBusy(null);
    }
  }

  return (
    <div>
      {error && <p className="mb-3 text-sm text-red-700">{error}</p>}
      <ul className="divide-y divide-line rounded-2xl border border-line bg-surface shadow-card">
        {members.map((m) => {
          const isAdmin = m.role === ROLE_ADMIN;
          const isSelf = m.id === currentUserId;
          const rowBusy = busy === m.id;

          return (
            <li key={m.id} className="flex items-center justify-between gap-4 px-6 py-4">
              <div>
                <p className="text-sm text-ink">{m.email}</p>
                <Badge tone={isAdmin ? "info" : "neutral"} className="mt-1">
                  {isAdmin ? "Customer admin" : "Member"}
                </Badge>
              </div>

              {confirming === m.id ? (
                <div className="flex items-center gap-2">
                  <span className="text-xs text-ink-muted">Offboard {m.email}?</span>
                  <Button size="sm" variant="primary" disabled={rowBusy} onClick={() => run(m.id, offboard)}>
                    {rowBusy ? "Offboarding…" : "Confirm"}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => setConfirming(null)}>
                    Cancel
                  </Button>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  {isAdmin ? (
                    <Button size="sm" variant="ghost" disabled={rowBusy || isSelf} onClick={() => run(m.id, demote)}>
                      Demote
                    </Button>
                  ) : (
                    <Button size="sm" variant="outline" disabled={rowBusy} onClick={() => run(m.id, promote)}>
                      Promote
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="ghost"
                    disabled={rowBusy || isSelf}
                    onClick={() => setConfirming(m.id)}
                  >
                    Offboard
                  </Button>
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
