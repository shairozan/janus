"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CodeBlock } from "@/components/ui/code-block";
import { Input, Label } from "@/components/ui/field";
import { buildLicensePublicKeyFile, EMBED_FILENAME, masterEmbedKeys } from "@/lib/embed-keys";
import type { OrgSummary, RotateKeyInput, RotateKeyResult, SigningKey } from "@/lib/types";

interface SigningKeysProps {
  keys: SigningKey[];
  orgs: OrgSummary[];
  rotate: (input: RotateKeyInput) => Promise<RotateKeyResult>;
}

const MASTER = "master";

function orgLabel(orgs: OrgSummary[], id?: number | null): string {
  if (id == null) return "Master (global)";
  const org = orgs.find((o) => o.id === id);

  return org ? `${org.name}` : `Org #${id}`;
}

function fmtDate(iso: string): string {
  const d = new Date(iso);

  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleDateString();
}

export function SigningKeys({ keys, orgs, rotate }: SigningKeysProps) {
  const router = useRouter();
  const [scope, setScope] = useState<string>(MASTER);
  const [days, setDays] = useState<string>("365");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [issued, setIssued] = useState<RotateKeyResult | null>(null);

  const embedKeys = masterEmbedKeys(keys);

  function downloadEmbedFile() {
    const blob = new Blob([buildLicensePublicKeyFile(keys)], { type: "application/x-pem-file" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = EMBED_FILENAME;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  async function runRotate() {
    setBusy(true);
    setError(null);
    setIssued(null);
    try {
      const organization_id = scope === MASTER ? undefined : Number(scope);
      const expires_in_days = days.trim() === "" ? undefined : Number(days);
      if (expires_in_days != null && (Number.isNaN(expires_in_days) || expires_in_days <= 0)) {
        throw new Error("Validity must be a positive number of days");
      }
      setIssued(await rotate({ organization_id, expires_in_days }));
      // Pull the fresh server-rendered list so the new key shows in the table and
      // is included in the downloadable embed file (the keys prop is otherwise
      // the page-load snapshot).
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Rotation failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Rotate / create a signing key</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-ink-muted text-pretty">
            Creates a fresh key for the chosen scope and retires the prior active one. Existing
            licenses signed by the retired key remain valid until they expire — rotation only
            changes what new licenses are signed with.
          </p>

          <div className="grid gap-4 sm:grid-cols-[1fr_auto_auto] sm:items-end">
            <div>
              <Label htmlFor="key-scope">Scope</Label>
              <select
                id="key-scope"
                value={scope}
                onChange={(e) => setScope(e.target.value)}
                className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-ink focus-visible:border-gold focus-visible:outline-none"
              >
                <option value={MASTER}>Master key (global)</option>
                {orgs.map((o) => (
                  <option key={o.id} value={String(o.id)}>
                    {o.name} · {o.customer_id}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <Label htmlFor="key-days">Validity (days)</Label>
              <Input
                id="key-days"
                type="number"
                min={1}
                value={days}
                onChange={(e) => setDays(e.target.value)}
                className="mt-1 w-32"
              />
            </div>

            <Button onClick={runRotate} disabled={busy}>
              {busy ? "Rotating…" : "Rotate key"}
            </Button>
          </div>

          {error && <p className="text-sm text-red-700">{error}</p>}

          {issued && (
            <div className="space-y-3 rounded-xl border border-gold/40 bg-gold/5 p-4">
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
                <Badge tone="success">New key</Badge>
                <span className="font-mono text-xs text-ink">{issued.new_key_id}</span>
                {issued.old_key_id && (
                  <span className="text-xs text-ink-muted">
                    retired <span className="font-mono">{issued.old_key_id}</span>
                  </span>
                )}
                <span className="text-xs text-ink-muted">expires {fmtDate(issued.expires_at)}</span>
              </div>
              <CodeBlock label="Public key (PEM)" value={issued.public_key} />
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <CardTitle>Signing keys</CardTitle>
            <p className="mt-1 text-sm text-ink-muted text-pretty">
              Download <span className="font-mono text-xs">{EMBED_FILENAME}</span> — the public keys
              of every non-revoked global key, concatenated, for the Janus build to embed. Public
              keys only; private halves never leave the server. Save it in the repo root as{" "}
              <span className="font-mono text-xs">{EMBED_FILENAME}</span> (your browser may drop the
              leading dot — restore it).
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={downloadEmbedFile}
            disabled={embedKeys.length === 0}
            title={
              embedKeys.length === 0
                ? "No global keys to embed yet"
                : `Includes ${embedKeys.length} global key${embedKeys.length === 1 ? "" : "s"}`
            }
          >
            Download {EMBED_FILENAME}
          </Button>
        </CardHeader>
        <CardContent className="p-0">
          {keys.length === 0 ? (
            <p className="px-6 py-8 text-center text-sm text-ink-muted">
              No signing keys yet. Rotate one above to seed the JWKS.
            </p>
          ) : (
            <ul className="divide-y divide-line">
              {keys.map((k) => {
                const revoked = k.revoked_at != null;
                const tone = revoked ? "danger" : k.active ? "success" : "neutral";
                const status = revoked ? "Revoked" : k.active ? "Active" : "Retired";

                return (
                  <li key={k.key_id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-6 py-4">
                    <span className="font-mono text-xs text-ink">{k.key_id}</span>
                    <Badge tone={tone}>{status}</Badge>
                    <span className="text-xs text-ink-muted">{orgLabel(orgs, k.organization_id)}</span>
                    <span className="ml-auto text-xs text-ink-muted">
                      created {fmtDate(k.created_at)} · expires {fmtDate(k.expires_at)}
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
