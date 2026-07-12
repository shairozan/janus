"use client";

import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox, Input, Label, Textarea } from "@/components/ui/field";
import { looksLikePublicKeyPem } from "@/lib/pem";
import type { PublicKey, ReplaceKeyInput } from "@/lib/types";

interface KeyManagerProps {
  active: PublicKey | null;
  history: PublicKey[];
  onReplace: (input: ReplaceKeyInput) => Promise<unknown>;
}

// Single active public key, GitHub-style. Uploading a key when one already
// exists is a REPLACEMENT: it invalidates run-log signature continuity and
// requires updating the local private key, so we show the caveat and require an
// explicit acknowledgement before enabling submit. A first upload has nothing to
// invalidate, so no caveat.
export function KeyManager({ active, history, onReplace }: KeyManagerProps) {
  const isReplacement = active !== null;

  const [pem, setPem] = useState("");
  const [title, setTitle] = useState("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const pemValid = looksLikePublicKeyPem(pem);
  const canSubmit = pemValid && (!isReplacement || acknowledged) && !submitting;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;

    setSubmitting(true);
    setError(null);
    try {
      await onReplace({ publicKeyPem: pem.trim(), title: title.trim() });
      setPem("");
      setTitle("");
      setAcknowledged(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to set key");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Signing key</CardTitle>
        <CardDescription>
          The public key bound to your license. You sign run logs locally with the matching private
          key; Janus verifies them with this one.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {active ? (
          <div className="rounded-lg border border-line bg-white px-4 py-3 text-sm">
            <div className="font-mono text-xs text-ink">{active.fingerprint}</div>
            <div className="mt-1 text-ink-muted">
              {active.title ? `${active.title} · ` : ""}active key
            </div>
          </div>
        ) : (
          <p className="text-sm text-ink-muted">
            No active key yet. Upload your RSA public key to receive a license.
          </p>
        )}

        <form onSubmit={handleSubmit} className="space-y-4" aria-label="key form">
          <div className="space-y-1.5">
            <Label htmlFor="pem">{isReplacement ? "New public key (PEM)" : "Public key (PEM)"}</Label>
            <Textarea
              id="pem"
              value={pem}
              onChange={(e) => setPem(e.target.value)}
              placeholder="-----BEGIN PUBLIC KEY-----&#10;...&#10;-----END PUBLIC KEY-----"
              aria-invalid={pem.length > 0 && !pemValid}
            />
            {pem.length > 0 && !pemValid && (
              <p className="text-xs text-red-700">That doesn&apos;t look like a PUBLIC KEY block.</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="title">Label (optional)</Label>
            <Input
              id="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="work laptop"
            />
          </div>

          {isReplacement && (
            <Alert tone="warning">
              <AlertTitle>Replacing your key reissues your license</AlertTitle>
              <p className="mt-1">
                Run logs signed with your old private key will no longer verify under your new
                license. You&apos;ll need to install the new license file <strong>and</strong> point
                your local Janus configuration (<code>signing.private_key_path</code>) at the
                matching private key.
              </p>
              <label className="mt-3 flex items-start gap-2">
                <Checkbox
                  checked={acknowledged}
                  onChange={(e) => setAcknowledged(e.target.checked)}
                  aria-label="I understand the consequences of replacing my key"
                />
                <span className="text-[13px]">I understand and want to replace my key.</span>
              </label>
            </Alert>
          )}

          {error && <p className="text-sm text-red-700">{error}</p>}

          <Button type="submit" disabled={!canSubmit}>
            {submitting ? "Saving…" : isReplacement ? "Replace key" : "Add key"}
          </Button>
        </form>

        {history.length > 0 && (
          <div className="border-t border-line pt-4">
            <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">
              Previous keys
            </p>
            <ul className="mt-2 space-y-1">
              {history.map((k) => (
                <li key={k.id} className="font-mono text-xs text-ink-muted">
                  {k.fingerprint} <span className="font-sans">(revoked)</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
