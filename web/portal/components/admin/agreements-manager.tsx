"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input, Label } from "@/components/ui/field";
import type {
  AgreementSummary,
  CreateAgreementInput,
  LicenseTier,
  OrgSummary,
} from "@/lib/types";

interface AgreementsManagerProps {
  agreements: AgreementSummary[];
  orgs: OrgSummary[];
  licenses: LicenseTier[];
  create: (input: CreateAgreementInput) => Promise<AgreementSummary>;
}

function orgName(orgs: OrgSummary[], id: number): string {
  return orgs.find((o) => o.id === id)?.name ?? `Org #${id}`;
}

function fmtDate(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);

  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleDateString();
}

function isoDay(d: Date): string {
  return d.toISOString().slice(0, 10);
}

export function AgreementsManager({ agreements, orgs, licenses, create }: AgreementsManagerProps) {
  const router = useRouter();
  const [orgID, setOrgID] = useState(orgs[0] ? String(orgs[0].id) : "");
  const [tier, setTier] = useState(licenses[0]?.tier ?? "");
  const [seats, setSeats] = useState("5");
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [cost, setCost] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [created, setCreated] = useState<AgreementSummary | null>(null);

  // Default the term client-side (today → +1 year, the A10 default) to avoid an
  // SSR/CSR hydration mismatch on date values.
  useEffect(() => {
    const today = new Date();
    const nextYear = new Date(today);
    nextYear.setFullYear(today.getFullYear() + 1);
    setStart(isoDay(today));
    setEnd(isoDay(nextYear));
  }, []);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setCreated(null);
    try {
      if (!orgID) throw new Error("Select an organization");
      if (!tier) throw new Error("Select a tier");
      const max_seats = Number(seats);
      if (Number.isNaN(max_seats) || max_seats <= 0) throw new Error("Seats must be a positive number");
      if (!start) throw new Error("A start date is required");

      const features = licenses.find((l) => l.tier === tier)?.features;
      const cost_per_month = cost.trim() === "" ? undefined : Number(cost);
      if (cost_per_month != null && (Number.isNaN(cost_per_month) || cost_per_month < 0)) {
        throw new Error("Cost must be a non-negative number");
      }

      const agreement = await create({
        organization_id: Number(orgID),
        tier,
        max_seats,
        start_date: start,
        end_date: end.trim() === "" ? undefined : end,
        features,
        cost_per_month,
      });
      setCreated(agreement);
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create agreement");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Define an agreement</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <Label htmlFor="agr-org">Organization</Label>
                <select
                  id="agr-org"
                  value={orgID}
                  onChange={(e) => setOrgID(e.target.value)}
                  className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-ink focus-visible:border-gold focus-visible:outline-none"
                >
                  {orgs.length === 0 && <option value="">No organizations</option>}
                  {orgs.map((o) => (
                    <option key={o.id} value={String(o.id)}>
                      {o.name} · {o.customer_id}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <Label htmlFor="agr-tier">Tier</Label>
                <select
                  id="agr-tier"
                  value={tier}
                  onChange={(e) => setTier(e.target.value)}
                  className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-ink focus-visible:border-gold focus-visible:outline-none"
                >
                  {licenses.length === 0 && <option value="">No tiers defined</option>}
                  {licenses.map((l) => (
                    <option key={l.id} value={l.tier}>
                      {l.name} ({l.tier})
                    </option>
                  ))}
                </select>
                {tier && (
                  <p className="mt-1 text-xs text-ink-muted">
                    Features: {licenses.find((l) => l.tier === tier)?.features.join(", ") || "—"}
                  </p>
                )}
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-3">
              <div>
                <Label htmlFor="agr-seats">Seats</Label>
                <Input
                  id="agr-seats"
                  type="number"
                  min={1}
                  value={seats}
                  onChange={(e) => setSeats(e.target.value)}
                  className="mt-1"
                />
              </div>
              <div>
                <Label htmlFor="agr-start">Start date</Label>
                <Input
                  id="agr-start"
                  type="date"
                  value={start}
                  onChange={(e) => setStart(e.target.value)}
                  className="mt-1"
                />
              </div>
              <div>
                <Label htmlFor="agr-end">End date</Label>
                <Input
                  id="agr-end"
                  type="date"
                  value={end}
                  onChange={(e) => setEnd(e.target.value)}
                  className="mt-1"
                />
              </div>
            </div>

            <div className="sm:w-1/3">
              <Label htmlFor="agr-cost">Cost / month (USD, optional)</Label>
              <Input
                id="agr-cost"
                type="number"
                min={0}
                step="0.01"
                value={cost}
                onChange={(e) => setCost(e.target.value)}
                placeholder="0 for comp"
                className="mt-1"
              />
            </div>

            {error && <p className="text-sm text-red-700">{error}</p>}

            <Button type="submit" disabled={busy}>
              {busy ? "Creating…" : "Create agreement"}
            </Button>

            {created && (
              <p className="text-sm text-ink">
                <Badge tone="success">Created</Badge>{" "}
                <span className="ml-1">
                  Agreement #{created.id} for {orgName(orgs, created.organization_id)} — issue a
                  license against it from <span className="font-medium">Issue License</span>.
                </span>
              </p>
            )}
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Agreements</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {agreements.length === 0 ? (
            <p className="px-6 py-8 text-center text-sm text-ink-muted">
              No agreements yet. Define one above to issue licenses against it.
            </p>
          ) : (
            <ul className="divide-y divide-line">
              {agreements.map((a) => (
                <li key={a.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-6 py-4">
                  <span className="font-mono text-xs text-ink">#{a.id}</span>
                  <Badge tone={a.deactivated_at ? "neutral" : "info"}>{a.tier || "—"}</Badge>
                  <span className="text-sm text-ink">{orgName(orgs, a.organization_id)}</span>
                  <span className="text-xs text-ink-muted">{a.max_seats} seats</span>
                  <span className="ml-auto text-xs text-ink-muted">
                    {fmtDate(a.start_date)} → {fmtDate(a.end_date)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
