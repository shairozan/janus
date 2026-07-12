import { requireStaff } from "@/lib/guards";
import { listOrganizations, listSigningKeys } from "@/lib/staff-api";
import { rotateKeyAction } from "@/lib/staff-actions";
import { SigningKeys } from "@/components/admin/signing-keys";

export default async function AdminKeysPage() {
  await requireStaff();
  const [keys, orgs] = await Promise.all([listSigningKeys(), listOrganizations()]);

  return (
    <div className="space-y-6">
      <div>
        <p className="font-display text-[11px] uppercase tracking-[0.18em] text-gold">Janus Staff</p>
        <h1 className="font-display text-2xl text-ink">Signing Keys</h1>
        <p className="mt-1 text-sm text-ink-muted text-pretty">
          The keys that sign issued licenses, published through the JWKS. Rotate to mint a new
          active key; retired keys keep verifying licenses they already signed.
        </p>
      </div>
      <SigningKeys keys={keys} orgs={orgs} rotate={rotateKeyAction} />
    </div>
  );
}
