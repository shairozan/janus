import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { KeyManager } from "@/components/me/key-manager";
import { LicenseSection } from "@/components/me/license-section";
import { getKeys, getLicense, getLicenseRequests, getProfile } from "@/lib/portal-api";
import { replaceKeyAction } from "@/lib/me-actions";
import { ROLE_ADMIN } from "@/lib/types";

// Server Component: loads the caller's data through the BFF (apiFetch forwards
// the Bearer token), then composes the client components. Reading the session
// makes this route dynamic, so no build-time fetch.
export default async function MePage() {
  const [profile, keys, requests, license] = await Promise.all([
    getProfile(),
    getKeys(),
    getLicenseRequests(),
    getLicense(),
  ]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Your account</h1>
        <p className="mt-1 text-sm text-ink-muted">{profile.email}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Profile</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-x-10 gap-y-3 text-sm">
          <Field label="Email" value={profile.email} />
          <Field
            label="Role"
            value={profile.role === ROLE_ADMIN ? "Customer admin" : "Member"}
          />
          <div>
            <p className="text-xs uppercase tracking-wide text-ink-muted">Status</p>
            <p className="mt-1">
              <Badge tone={profile.has_license ? "success" : "neutral"}>
                {profile.has_license ? "Licensed" : "No license"}
              </Badge>
            </p>
          </div>
        </CardContent>
      </Card>

      <KeyManager active={keys.active} history={keys.history} onReplace={replaceKeyAction} />

      <LicenseSection license={license} requests={requests} />
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs uppercase tracking-wide text-ink-muted">{label}</p>
      <p className="mt-1 text-ink">{value}</p>
    </div>
  );
}
