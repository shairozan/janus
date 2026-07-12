"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label, Textarea } from "@/components/ui/field";
import type { RequestProposalInput } from "@/lib/types";

interface ProposalFormProps {
  submit: (input: RequestProposalInput) => Promise<unknown>;
  submitLabel: string;
  defaults?: Partial<RequestProposalInput>;
}

const LICENSE_MODELS = ["subscription", "concurrent", "named"] as const;

const selectClass =
  "w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-ink focus-visible:border-gold focus-visible:outline-none";

export function ProposalForm({ submit, submitLabel, defaults }: ProposalFormProps) {
  const [seats, setSeats] = useState(String(defaults?.seats ?? 10));
  const [licenseModel, setLicenseModel] = useState(defaults?.license_model ?? "subscription");
  const [tier, setTier] = useState(defaults?.tier ?? "");
  const [validityDays, setValidityDays] = useState(String(defaults?.validity_days ?? 365));
  const [message, setMessage] = useState(defaults?.message ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const seatCount = Number(seats);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const input: RequestProposalInput = {
        seats: seatCount,
        license_model: licenseModel,
        validity_days: Number(validityDays) || undefined,
      };
      if (tier.trim()) {
        input.tier = tier.trim();
      }

      if (message.trim()) {
        input.message = message.trim();
      }

      await submit(input);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not submit");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="grid gap-2">
          <Label htmlFor="pf-seats">Seats</Label>
          <Input
            id="pf-seats"
            type="number"
            min={1}
            value={seats}
            onChange={(e) => setSeats(e.target.value)}
          />
        </div>

        <div className="grid gap-2">
          <Label htmlFor="pf-model">License model</Label>
          <select
            id="pf-model"
            value={licenseModel}
            onChange={(e) => setLicenseModel(e.target.value)}
            className={selectClass}
          >
            {LICENSE_MODELS.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="pf-tier">Tier (optional)</Label>
          <Input
            id="pf-tier"
            value={tier}
            onChange={(e) => setTier(e.target.value)}
            placeholder="e.g. enterprise"
          />
        </div>

        <div className="grid gap-2">
          <Label htmlFor="pf-validity">Term (days)</Label>
          <Input
            id="pf-validity"
            type="number"
            min={1}
            value={validityDays}
            onChange={(e) => setValidityDays(e.target.value)}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="pf-message">Note (optional)</Label>
        <Textarea
          id="pf-message"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          className="font-sans text-sm"
          placeholder="Anything Janus should know about this request"
        />
      </div>

      {error && <p className="text-sm text-red-700">{error}</p>}

      <Button type="submit" variant="primary" disabled={busy || seatCount <= 0}>
        {busy ? "Submitting…" : submitLabel}
      </Button>
    </form>
  );
}
