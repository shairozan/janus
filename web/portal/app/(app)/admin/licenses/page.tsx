import { requireStaff } from "@/lib/guards";
import { listAgreements, listOrganizations } from "@/lib/staff-api";
import { issueLicenseAction } from "@/lib/staff-actions";
import { IssueLicense } from "@/components/admin/issue-license";

export default async function AdminLicensesPage() {
  await requireStaff();
  const [orgs, agreements] = await Promise.all([listOrganizations(), listAgreements()]);

  return (
    <div className="space-y-6">
      <div>
        <p className="font-display text-[11px] uppercase tracking-[0.18em] text-gold">Janus Staff</p>
        <h1 className="font-display text-2xl text-ink">Issue License</h1>
        <p className="mt-1 text-sm text-ink-muted text-pretty">
          Mint a license JWT for a user against an agreement, signed by the org&apos;s active
          signing key. Use this for comp / demo licenses that don&apos;t flow through the normal
          request queue.
        </p>
      </div>
      <IssueLicense orgs={orgs} agreements={agreements} issue={issueLicenseAction} />
    </div>
  );
}
