"use client";

import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CodeBlock } from "@/components/ui/code-block";
import { Input, Label, Textarea } from "@/components/ui/field";
import { KeyGenInstructions } from "@/components/key-gen-instructions";
import type { AgreementSummary, IssuedLicense, IssueLicenseInput, OrgSummary } from "@/lib/types";

interface IssueLicenseProps {
  orgs: OrgSummary[];
  agreements: AgreementSummary[];
  issue: (input: IssueLicenseInput) => Promise<IssuedLicense>;
}

function fmtDate(iso: string): string {
  const d = new Date(iso);

  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleString();
}

export function IssueLicense({ orgs, agreements, issue }: IssueLicenseProps) {
  const [orgID, setOrgID] = useState<string>(orgs[0] ? String(orgs[0].id) : "");
  const [agreementID, setAgreementID] = useState<string>("");
  const [email, setEmail] = useState("");
  const [days, setDays] = useState("");
  const [pem, setPem] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [issued, setIssued] = useState<IssuedLicense | null>(null);

  // Agreements for the selected org (the picker is scoped so staff don't bind a
  // license to the wrong org's agreement).
  const orgAgreements = useMemo(
    () => agreements.filter((a) => String(a.organization_id) === orgID && a.deactivated_at == null),
    [agreements, orgID]
  );

  function onOrgChange(value: string) {
    setOrgID(value);
    setAgreementID(""); // reset — agreements are org-scoped
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setIssued(null);
    try {
      if (!agreementID) throw new Error("Choose an agreement to issue against");
      if (!email.trim()) throw new Error("User email is required");

      let duration_seconds: number | undefined;
      if (days.trim() !== "") {
        const n = Number(days);
        if (Number.isNaN(n) || n <= 0) throw new Error("Duration must be a positive number of days");
        duration_seconds = Math.round(n * 86400);
      }

      setIssued(
        await issue({
          agreement_id: Number(agreementID),
          user_email: email.trim(),
          duration_seconds,
          signing_public_key: pem.trim() === "" ? undefined : pem.trim(),
        })
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to issue license");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Issue a license</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <Label htmlFor="lic-org">Organization</Label>
                <select
                  id="lic-org"
                  value={orgID}
                  onChange={(e) => onOrgChange(e.target.value)}
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
                <Label htmlFor="lic-agreement">Agreement</Label>
                <select
                  id="lic-agreement"
                  value={agreementID}
                  onChange={(e) => setAgreementID(e.target.value)}
                  className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-ink focus-visible:border-gold focus-visible:outline-none"
                >
                  <option value="">Select an agreement…</option>
                  {orgAgreements.map((a) => (
                    <option key={a.id} value={String(a.id)}>
                      #{a.id} · {a.tier || "—"} · {a.max_seats} seats
                    </option>
                  ))}
                </select>
                {orgID && orgAgreements.length === 0 && (
                  <p className="mt-1 text-xs text-ink-muted">No active agreements for this org.</p>
                )}
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <Label htmlFor="lic-email">User email</Label>
                <Input
                  id="lic-email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="user@customer.com"
                  className="mt-1"
                />
              </div>

              <div>
                <Label htmlFor="lic-days">Duration (days, optional)</Label>
                <Input
                  id="lic-days"
                  type="number"
                  min={1}
                  value={days}
                  onChange={(e) => setDays(e.target.value)}
                  placeholder="defaults to agreement term"
                  className="mt-1"
                />
              </div>
            </div>

            <div>
              <Label htmlFor="lic-pem">Signing public key — PEM (optional)</Label>
              <Textarea
                id="lic-pem"
                value={pem}
                onChange={(e) => setPem(e.target.value)}
                placeholder="-----BEGIN PUBLIC KEY-----"
                className="mt-1"
              />
              <p className="mt-1 text-xs text-ink-muted text-pretty">
                The user&apos;s RSA public key, embedded in the license so Janus can verify their
                signed run logs. Leave blank to issue a license without an embedded key.
              </p>
              <div className="mt-2">
                <KeyGenInstructions />
              </div>
            </div>

            {error && <p className="text-sm text-red-700">{error}</p>}

            <Button type="submit" disabled={busy}>
              {busy ? "Issuing…" : "Issue license"}
            </Button>
          </form>
        </CardContent>
      </Card>

      {issued && (
        <Card>
          <CardHeader>
            <CardTitle>License issued</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-3 text-sm">
              <Badge tone="success">Signed</Badge>
              <span className="text-xs text-ink-muted">expires {fmtDate(issued.expires_at)}</span>
            </div>
            <CodeBlock label="License JWT" value={issued.token} />
          </CardContent>
        </Card>
      )}
    </div>
  );
}
