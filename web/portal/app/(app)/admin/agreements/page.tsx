import { requireStaff } from "@/lib/guards";
import { listAgreements, listLicenses, listOrganizations } from "@/lib/staff-api";
import { createAgreementAction } from "@/lib/staff-actions";
import { AgreementsManager } from "@/components/admin/agreements-manager";

export default async function AdminAgreementsPage() {
  await requireStaff();
  const [agreements, orgs, licenses] = await Promise.all([
    listAgreements(),
    listOrganizations(),
    listLicenses(),
  ]);

  return (
    <div className="space-y-6">
      <div>
        <p className="font-display text-[11px] uppercase tracking-[0.18em] text-gold">Janus Staff</p>
        <h1 className="font-display text-2xl text-ink">Agreements</h1>
        <p className="mt-1 text-sm text-ink-muted text-pretty">
          Define an agreement directly — for internal, comp, or demo use — without the customer
          proposal and payment flow. Then issue licenses against it from Issue License.
        </p>
      </div>
      <AgreementsManager
        agreements={agreements}
        orgs={orgs}
        licenses={licenses}
        create={createAgreementAction}
      />
    </div>
  );
}
