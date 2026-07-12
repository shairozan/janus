"use client";

import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/field";
import type { AgreementUsage, SeatPreview } from "@/lib/types";

interface SeatsManagerProps {
  agreements: AgreementUsage[];
  preview: (agreementID: number, add: number) => Promise<SeatPreview>;
  add: (agreementID: number, add: number) => Promise<AgreementUsage>;
}

function dollars(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

function AgreementRow({
  agreement,
  preview,
  add,
}: {
  agreement: AgreementUsage;
  preview: SeatsManagerProps["preview"];
  add: SeatsManagerProps["add"];
}) {
  const [count, setCount] = useState("");
  const [quote, setQuote] = useState<SeatPreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const full = agreement.used_seats >= agreement.seats;
  const addN = Number(count);

  async function runPreview() {
    setBusy(true);
    setError(null);
    try {
      setQuote(await preview(agreement.id, addN));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load preview");
    } finally {
      setBusy(false);
    }
  }

  async function confirm() {
    setBusy(true);
    setError(null);
    try {
      await add(agreement.id, addN);
      setQuote(null);
      setCount("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not add seats");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="rounded-2xl border border-line bg-surface p-6 shadow-card">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="font-display text-lg text-ink">
            {agreement.tier ?? agreement.license_model} agreement
          </p>
          <p className="mt-1 text-sm text-ink-muted">
            {agreement.used_seats} / {agreement.seats} seats in use
          </p>
        </div>
        <Badge tone={full ? "danger" : "info"}>{dollars(agreement.price_per_seat_cents)}/seat</Badge>
      </div>

      {full && (
        <Alert tone="warning" className="mt-4">
          No seats available — add seats below to unblock pending requests.
        </Alert>
      )}

      {error && <p className="mt-4 text-sm text-red-700">{error}</p>}

      <div className="mt-4 flex flex-wrap items-end gap-3">
        <div className="grid gap-2">
          <Label htmlFor={`add-${agreement.id}`}>Add seats</Label>
          <Input
            id={`add-${agreement.id}`}
            type="number"
            min={1}
            value={count}
            onChange={(e) => {
              setCount(e.target.value);
              setQuote(null);
            }}
            className="w-28"
            placeholder="0"
          />
        </div>
        <Button variant="outline" disabled={busy || addN <= 0} onClick={runPreview}>
          Preview
        </Button>
      </div>

      {quote && (
        <div className="mt-4 space-y-3 rounded-lg border border-line bg-paper/60 p-4">
          <dl className="grid grid-cols-2 gap-1 text-sm">
            <dt className="text-ink-muted">New total</dt>
            <dd className="text-right text-ink">{quote.new_seats} seats</dd>
            <dt className="text-ink-muted">Prorated charge now</dt>
            <dd className="text-right text-ink">{dollars(quote.prorated_charge_cents)}</dd>
            <dt className="text-ink-muted">Recurring increase</dt>
            <dd className="text-right text-ink">{dollars(quote.recurring_delta_cents)}/cycle</dd>
          </dl>
          <Button variant="primary" disabled={busy} onClick={confirm}>
            {busy ? "Adding…" : "Confirm & add"}
          </Button>
        </div>
      )}
    </div>
  );
}

export function SeatsManager({ agreements, preview, add }: SeatsManagerProps) {
  if (agreements.length === 0) {
    return <p className="text-sm text-ink-muted">No active agreements.</p>;
  }

  return (
    <div className="space-y-6">
      {agreements.map((a) => (
        <AgreementRow key={a.id} agreement={a} preview={preview} add={add} />
      ))}
    </div>
  );
}
