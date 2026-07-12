"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/field";
import type { LicenseRequest } from "@/lib/types";

interface RequestQueueProps {
  requests: LicenseRequest[];
  approve: (reqID: number) => Promise<void>;
  reject: (reqID: number, reason: string) => Promise<void>;
}

const statusTone: Record<string, "success" | "warning" | "danger" | "neutral"> = {
  approved: "success",
  pending: "warning",
  rejected: "danger",
  failed: "danger",
};

export function RequestQueue({ requests, approve, reject }: RequestQueueProps) {
  const [busy, setBusy] = useState<number | null>(null);
  const [rejecting, setRejecting] = useState<number | null>(null);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function act(reqID: number, fn: () => Promise<void>) {
    setBusy(reqID);
    setError(null);
    try {
      await fn();
      setRejecting(null);
      setReason("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Action failed");
    } finally {
      setBusy(null);
    }
  }

  const pending = requests.filter((r) => r.status === "pending");
  const decided = requests.filter((r) => r.status !== "pending");

  return (
    <div className="space-y-8">
      {error && <p className="text-sm text-red-700">{error}</p>}

      <section>
        <h2 className="font-display text-lg text-ink">Pending ({pending.length})</h2>
        {pending.length === 0 ? (
          <p className="mt-2 text-sm text-ink-muted">No pending requests.</p>
        ) : (
          <ul className="mt-3 divide-y divide-line rounded-2xl border border-line bg-surface shadow-card">
            {pending.map((r) => (
              <li key={r.id} className="px-6 py-4">
                <div className="flex items-center justify-between gap-4">
                  <span className="text-sm text-ink">
                    Request #{r.id}
                    {r.reason ? <span className="text-ink-muted"> — {r.reason}</span> : null}
                  </span>
                  {rejecting === r.id ? null : (
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        variant="primary"
                        disabled={busy === r.id}
                        onClick={() => act(r.id, () => approve(r.id))}
                      >
                        Approve
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => setRejecting(r.id)}>
                        Reject
                      </Button>
                    </div>
                  )}
                </div>

                {rejecting === r.id && (
                  <div className="mt-3 flex items-center gap-2">
                    <Input
                      value={reason}
                      onChange={(e) => setReason(e.target.value)}
                      placeholder="Reason (optional)"
                      aria-label="rejection reason"
                    />
                    <Button
                      size="sm"
                      variant="primary"
                      disabled={busy === r.id}
                      onClick={() => act(r.id, () => reject(r.id, reason))}
                    >
                      Confirm reject
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setRejecting(null)}>
                      Cancel
                    </Button>
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      {decided.length > 0 && (
        <section>
          <h2 className="font-display text-lg text-ink">History</h2>
          <ul className="mt-3 divide-y divide-line rounded-2xl border border-line bg-surface shadow-card">
            {decided.map((r) => (
              <li key={r.id} className="flex items-center justify-between px-6 py-3 text-sm">
                <span className="text-ink-muted">Request #{r.id}</span>
                <Badge tone={statusTone[r.status] ?? "neutral"}>{r.status}</Badge>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
