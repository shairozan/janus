"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/field";
import {
  RULE_EMAIL_DOMAIN,
  RULE_SEAT_THRESHOLD,
  type AutoAcceptanceRule,
  type CreateRuleInput,
} from "@/lib/types";

interface RulesManagerProps {
  rules: AutoAcceptanceRule[];
  create: (input: CreateRuleInput) => Promise<void>;
  setEnabled: (ruleID: number, enabled: boolean) => Promise<void>;
  remove: (ruleID: number) => Promise<void>;
}

function describeRule(r: AutoAcceptanceRule): string {
  if (r.rule_type === RULE_SEAT_THRESHOLD) {
    return `Auto-approve up to ${r.max_auto_seats ?? 0} seats`;
  }

  return `Auto-approve @${r.match_domain ?? ""}`;
}

export function RulesManager({ rules, create, setEnabled, remove }: RulesManagerProps) {
  const [ruleType, setRuleType] = useState<string>(RULE_EMAIL_DOMAIN);
  const [domain, setDomain] = useState("");
  const [maxSeats, setMaxSeats] = useState("");
  const [busy, setBusy] = useState(false);
  const [rowBusy, setRowBusy] = useState<number | null>(null);
  const [confirming, setConfirming] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const input: CreateRuleInput =
        ruleType === RULE_SEAT_THRESHOLD
          ? { rule_type: RULE_SEAT_THRESHOLD, max_auto_seats: Number(maxSeats) }
          : { rule_type: RULE_EMAIL_DOMAIN, match_domain: domain.trim() };
      await create(input);
      setDomain("");
      setMaxSeats("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create rule");
    } finally {
      setBusy(false);
    }
  }

  async function row(ruleID: number, fn: () => Promise<void>) {
    setRowBusy(ruleID);
    setError(null);
    try {
      await fn();
      setConfirming(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Action failed");
    } finally {
      setRowBusy(null);
    }
  }

  const canSubmit =
    ruleType === RULE_SEAT_THRESHOLD ? Number(maxSeats) > 0 : domain.trim().length > 0;

  return (
    <div className="space-y-8">
      {error && <p className="text-sm text-red-700">{error}</p>}

      <ul className="divide-y divide-line rounded-2xl border border-line bg-surface shadow-card">
        {rules.length === 0 ? (
          <li className="px-6 py-5 text-sm text-ink-muted">
            No rules yet — every license request is reviewed manually.
          </li>
        ) : (
          rules.map((r) => (
            <li key={r.id} className="flex items-center justify-between gap-4 px-6 py-4">
              <div>
                <p className="text-sm text-ink">{describeRule(r)}</p>
                <Badge tone={r.enabled ? "success" : "neutral"} className="mt-1">
                  {r.enabled ? "Enabled" : "Disabled"}
                </Badge>
              </div>

              {confirming === r.id ? (
                <div className="flex items-center gap-2">
                  <span className="text-xs text-ink-muted">Delete this rule?</span>
                  <Button
                    size="sm"
                    variant="primary"
                    disabled={rowBusy === r.id}
                    onClick={() => row(r.id, () => remove(r.id))}
                  >
                    {rowBusy === r.id ? "Deleting…" : "Confirm"}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => setConfirming(null)}>
                    Cancel
                  </Button>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={rowBusy === r.id}
                    onClick={() => row(r.id, () => setEnabled(r.id, !r.enabled))}
                  >
                    {r.enabled ? "Disable" : "Enable"}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => setConfirming(r.id)}>
                    Delete
                  </Button>
                </div>
              )}
            </li>
          ))
        )}
      </ul>

      <form onSubmit={submit} className="space-y-4 rounded-2xl border border-line bg-surface p-6 shadow-card">
        <h2 className="font-display text-lg text-ink">Add a rule</h2>

        <div className="grid gap-2">
          <Label htmlFor="rule-type">Rule type</Label>
          <select
            id="rule-type"
            value={ruleType}
            onChange={(e) => setRuleType(e.target.value)}
            className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-ink focus-visible:border-gold focus-visible:outline-none"
          >
            <option value={RULE_EMAIL_DOMAIN}>Email domain</option>
            <option value={RULE_SEAT_THRESHOLD}>Seat threshold</option>
          </select>
        </div>

        {ruleType === RULE_EMAIL_DOMAIN ? (
          <div className="grid gap-2">
            <Label htmlFor="rule-domain">Email domain</Label>
            <Input
              id="rule-domain"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="acme.com"
            />
            <p className="text-xs text-ink-muted">
              Requests from members with this email domain are approved automatically.
            </p>
          </div>
        ) : (
          <div className="grid gap-2">
            <Label htmlFor="rule-seats">Max auto-approved seats</Label>
            <Input
              id="rule-seats"
              type="number"
              min={1}
              value={maxSeats}
              onChange={(e) => setMaxSeats(e.target.value)}
              placeholder="5"
            />
            <p className="text-xs text-ink-muted">
              Auto-approve until this many seats are in use, then fall back to manual review.
            </p>
          </div>
        )}

        <Button type="submit" variant="primary" disabled={busy || !canSubmit}>
          {busy ? "Adding…" : "Add rule"}
        </Button>
      </form>
    </div>
  );
}
