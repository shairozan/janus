"use client";

import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ProposalForm } from "./proposal-form";
import {
  PROPOSAL_ACCEPTED,
  PROPOSAL_COUNTERED,
  PROPOSAL_EXPIRED,
  PROPOSAL_FULFILLED,
  PROPOSAL_PAID,
  PROPOSAL_PROPOSED,
  PROPOSAL_REJECTED,
  PROPOSAL_REQUESTED,
  type AgreementProposal,
  type RequestProposalInput,
} from "@/lib/types";

interface BillingScreenProps {
  proposals: AgreementProposal[];
  request: (input: RequestProposalInput) => Promise<unknown>;
  counter: (pid: number, input: RequestProposalInput) => Promise<unknown>;
  accept: (pid: number) => Promise<unknown>;
  pay: (pid: number) => Promise<unknown>;
}

const statusTone: Record<string, "success" | "warning" | "danger" | "info" | "neutral"> = {
  [PROPOSAL_REQUESTED]: "neutral",
  [PROPOSAL_COUNTERED]: "neutral",
  [PROPOSAL_PROPOSED]: "info",
  [PROPOSAL_ACCEPTED]: "warning",
  [PROPOSAL_PAID]: "success",
  [PROPOSAL_FULFILLED]: "success",
  [PROPOSAL_REJECTED]: "danger",
  [PROPOSAL_EXPIRED]: "danger",
};

function money(cents: number, currency = "usd"): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: currency.toUpperCase(),
  }).format(cents / 100);
}

function ProposalActions({
  proposal,
  counter,
  accept,
  pay,
}: {
  proposal: AgreementProposal;
  counter: BillingScreenProps["counter"];
  accept: BillingScreenProps["accept"];
  pay: BillingScreenProps["pay"];
}) {
  const [countering, setCountering] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    setError(null);
    try {
      await fn();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Action failed");
    } finally {
      setBusy(false);
    }
  }

  if (proposal.status === PROPOSAL_REQUESTED || proposal.status === PROPOSAL_COUNTERED) {
    return <p className="text-sm text-ink-muted">Awaiting an offer from Janus.</p>;
  }

  if (proposal.status === PROPOSAL_PAID || proposal.status === PROPOSAL_FULFILLED) {
    return (
      <Alert tone="info">
        Agreement active — members can now request seats from their profile.
      </Alert>
    );
  }

  if (proposal.status === PROPOSAL_ACCEPTED) {
    return (
      <div className="space-y-2">
        {error && <p className="text-sm text-red-700">{error}</p>}
        <p className="text-sm text-ink-muted">
          Accepted — pay to start the subscription and activate the agreement.
        </p>
        <Button variant="primary" disabled={busy} onClick={() => run(() => pay(proposal.id))}>
          {busy ? "Processing…" : `Pay ${money(proposal.total_amount_cents)}`}
        </Button>
      </div>
    );
  }

  if (proposal.status !== PROPOSAL_PROPOSED) {
    return null;
  }

  return (
    <div className="space-y-4">
      {error && <p className="text-sm text-red-700">{error}</p>}

      {countering ? (
        <div className="rounded-lg border border-line bg-paper/60 p-4">
          <p className="mb-3 font-display text-sm text-ink">Counter offer</p>
          <ProposalForm
            submitLabel="Send counter"
            defaults={{
              seats: proposal.seats,
              license_model: proposal.license_model,
              tier: proposal.tier ?? undefined,
              validity_days: proposal.validity_days,
            }}
            submit={async (input) => {
              await counter(proposal.id, input);
              setCountering(false);
            }}
          />
          <Button variant="ghost" size="sm" className="mt-2" onClick={() => setCountering(false)}>
            Cancel
          </Button>
        </div>
      ) : (
        <div className="flex gap-2">
          <Button variant="primary" disabled={busy} onClick={() => run(() => accept(proposal.id))}>
            Accept offer
          </Button>
          <Button variant="outline" onClick={() => setCountering(true)}>
            Counter
          </Button>
        </div>
      )}
    </div>
  );
}

function ProposalCard({
  proposal,
  counter,
  accept,
  pay,
}: {
  proposal: AgreementProposal;
  counter: BillingScreenProps["counter"];
  accept: BillingScreenProps["accept"];
  pay: BillingScreenProps["pay"];
}) {
  const lineItems = proposal.line_items ?? [];
  const currency = lineItems[0]?.price?.currency ?? "usd";

  return (
    <Card>
      <CardHeader className="flex items-center justify-between">
        <CardTitle>
          {proposal.seats} seats · {proposal.license_model}
          {proposal.tier ? ` · ${proposal.tier}` : ""}
        </CardTitle>
        <Badge tone={statusTone[proposal.status] ?? "neutral"}>{proposal.status}</Badge>
      </CardHeader>
      <CardContent className="space-y-4">
        {proposal.message && (
          <p className="text-sm italic text-ink-muted">&ldquo;{proposal.message}&rdquo;</p>
        )}

        {lineItems.length > 0 && (
          <div className="overflow-hidden rounded-lg border border-line">
            <table className="w-full text-sm">
              <tbody className="divide-y divide-line">
                {lineItems.map((li) => (
                  <tr key={li.id}>
                    <td className="px-4 py-2 text-ink">{li.price?.nickname ?? `Price #${li.price_id}`}</td>
                    <td className="px-4 py-2 text-right text-ink-muted">× {li.quantity}</td>
                    <td className="px-4 py-2 text-right text-ink">{money(li.amount_cents, currency)}</td>
                  </tr>
                ))}
              </tbody>
              <tfoot>
                <tr className="border-t border-line bg-paper/60">
                  <td className="px-4 py-2 font-medium text-ink" colSpan={2}>
                    Total
                  </td>
                  <td className="px-4 py-2 text-right font-medium text-ink">
                    {money(proposal.total_amount_cents, currency)}
                  </td>
                </tr>
              </tfoot>
            </table>
          </div>
        )}

        <p className="text-xs text-ink-muted">Term: {proposal.validity_days} days</p>

        <ProposalActions proposal={proposal} counter={counter} accept={accept} pay={pay} />
      </CardContent>
    </Card>
  );
}

export function BillingScreen({ proposals, request, counter, accept, pay }: BillingScreenProps) {
  const inFlight = proposals.some(
    (p) => p.status !== PROPOSAL_REJECTED && p.status !== PROPOSAL_EXPIRED
  );

  return (
    <div className="space-y-8">
      {proposals.length > 0 && (
        <div className="space-y-4">
          {proposals.map((p) => (
            <ProposalCard key={p.id} proposal={p} counter={counter} accept={accept} pay={pay} />
          ))}
        </div>
      )}

      {!inFlight && (
        <Card>
          <CardHeader>
            <CardTitle>Request an agreement</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="mb-4 text-sm text-ink-muted">
              Tell us how many seats you need. Janus will send back a priced offer you can accept or
              counter.
            </p>
            <ProposalForm submit={request} submitLabel="Request agreement" />
          </CardContent>
        </Card>
      )}
    </div>
  );
}
