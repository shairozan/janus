import { requireAdmin } from "@/lib/guards";
import { listAgreements } from "@/lib/portal-api";
import { SeatsManager } from "@/components/admin/seats-manager";
import { addSeatsAction, previewSeatsAction } from "@/lib/admin-actions";

export default async function SeatsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const orgID = Number(id);
  await requireAdmin(orgID);
  const agreements = await listAgreements(orgID);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Seats</h1>
        <p className="mt-1 text-sm text-ink-muted">
          Track seat usage and add seats to an agreement. Adding seats previews the prorated charge
          before anything is billed.
        </p>
      </div>
      <SeatsManager
        agreements={agreements}
        preview={previewSeatsAction.bind(null, orgID)}
        add={addSeatsAction.bind(null, orgID)}
      />
    </div>
  );
}
