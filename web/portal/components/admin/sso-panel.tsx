"use client";

import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/field";
import type { SSOConfig, SSOSetupInput } from "@/lib/types";

interface SSOPanelProps {
  config: SSOConfig | null;
  setup: (input: SSOSetupInput) => Promise<SSOConfig>;
}

const statusTone: Record<string, "success" | "warning" | "danger"> = {
  active: "success",
  pending: "warning",
  failed: "danger",
};

function ReadOnlyView({ config }: { config: SSOConfig }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(config.login_url);
      setCopied(true);
    } catch {
      // Clipboard may be unavailable (e.g. insecure context) — the field is
      // selectable as a fallback, so this is non-fatal.
    }
  }

  return (
    <div className="space-y-6">
      <div className="rounded-2xl border border-line bg-surface p-6 shadow-card">
        <div className="flex items-center justify-between">
          <h2 className="font-display text-lg text-ink">{config.provider_name}</h2>
          <Badge tone={statusTone[config.cognito_status] ?? "neutral"}>{config.cognito_status}</Badge>
        </div>

        <dl className="mt-4 grid gap-x-6 gap-y-3 text-sm sm:grid-cols-[10rem_1fr]">
          <dt className="text-ink-muted">Client ID</dt>
          <dd className="font-mono text-ink">{config.oidc_client_id}</dd>
          <dt className="text-ink-muted">Issuer</dt>
          <dd className="break-all font-mono text-ink">{config.oidc_issuer}</dd>
          <dt className="text-ink-muted">Scopes</dt>
          <dd className="text-ink">{config.scopes}</dd>
          <dt className="text-ink-muted">Client secret</dt>
          <dd className="text-ink">
            {config.has_client_secret ? "Secret stored (write-only)" : "None"}
          </dd>
          <dt className="text-ink-muted">User pool</dt>
          <dd className="font-mono text-ink">{config.user_pool_id}</dd>
        </dl>
      </div>

      <div className="rounded-2xl border border-line bg-surface p-6 shadow-card">
        <Label htmlFor="sso-login-url">Login URL</Label>
        <p className="mt-1 text-xs text-ink-muted">Share this deep link with your users.</p>
        <div className="mt-2 flex gap-2">
          <Input id="sso-login-url" readOnly value={config.login_url} className="font-mono text-xs" />
          <Button variant="outline" type="button" onClick={copy}>
            {copied ? "Copied" : "Copy"}
          </Button>
        </div>
      </div>

      <Alert tone="info">
        Changing a live identity provider can lock out every SSO user in your org, so it is handled by
        support rather than self-service. Contact support to update or remove this configuration.
      </Alert>
    </div>
  );
}

function SetupWizard({ setup }: { setup: SSOPanelProps["setup"] }) {
  const [providerName, setProviderName] = useState("");
  const [issuer, setIssuer] = useState("");
  const [clientID, setClientID] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [scopes, setScopes] = useState("openid email profile");
  const [emailAttr, setEmailAttr] = useState("email");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await setup({
        provider_name: providerName.trim(),
        oidc_issuer: issuer.trim(),
        client_id: clientID.trim(),
        client_secret: clientSecret,
        scopes: scopes.trim(),
        attribute_map: { email: emailAttr.trim() },
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create SSO configuration");
    } finally {
      setBusy(false);
    }
  }

  const ready =
    providerName.trim() && issuer.trim() && clientID.trim() && clientSecret && scopes.trim();

  return (
    <form onSubmit={submit} className="space-y-5 rounded-2xl border border-line bg-surface p-6 shadow-card">
      <div>
        <h2 className="font-display text-lg text-ink">Set up SSO</h2>
        <p className="mt-1 text-sm text-ink-muted">
          Connect your OIDC identity provider. This is created once — later changes go through support.
        </p>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="provider-name">Provider name</Label>
        <Input
          id="provider-name"
          value={providerName}
          onChange={(e) => setProviderName(e.target.value)}
          placeholder="acme-okta"
          maxLength={32}
        />
        <p className="text-xs text-ink-muted">A short identifier (≤ 32 chars), used in the login URL.</p>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="issuer">OIDC issuer</Label>
        <Input
          id="issuer"
          value={issuer}
          onChange={(e) => setIssuer(e.target.value)}
          placeholder="https://acme.okta.com"
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="client-id">Client ID</Label>
        <Input id="client-id" value={clientID} onChange={(e) => setClientID(e.target.value)} />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="client-secret">Client secret</Label>
        <Input
          id="client-secret"
          type="password"
          value={clientSecret}
          onChange={(e) => setClientSecret(e.target.value)}
        />
        <p className="text-xs text-ink-muted">Stored encrypted and never shown again.</p>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="scopes">Scopes</Label>
        <Input id="scopes" value={scopes} onChange={(e) => setScopes(e.target.value)} />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="email-attr">Email attribute</Label>
        <Input id="email-attr" value={emailAttr} onChange={(e) => setEmailAttr(e.target.value)} />
        <p className="text-xs text-ink-muted">
          The provider claim that carries the user&apos;s email.
        </p>
      </div>

      {error && <p className="text-sm text-red-700">{error}</p>}

      <Button type="submit" variant="primary" disabled={busy || !ready}>
        {busy ? "Creating…" : "Create SSO"}
      </Button>
    </form>
  );
}

export function SSOPanel({ config, setup }: SSOPanelProps) {
  return config ? <ReadOnlyView config={config} /> : <SetupWizard setup={setup} />;
}
