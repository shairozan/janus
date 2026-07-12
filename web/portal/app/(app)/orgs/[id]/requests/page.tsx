import { requireAdmin } from "@/lib/guards";
import { listOrgRequests } from "@/lib/portal-api";
import { RequestQueue } from "@/components/admin/request-queue";
import { approveRequestAction, rejectRequestAction } from "@/lib/admin-actions";

export default async function RequestsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const orgID = Number(id);
  await requireAdmin(orgID);
  const requests = await listOrgRequests(orgID);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-2xl text-ink">License requests</h1>
        <p className="mt-1 text-sm text-ink-muted">Approve or reject members&apos; license requests.</p>
      </div>
      <RequestQueue
        requests={requests}
        approve={approveRequestAction.bind(null, orgID)}
        reject={rejectRequestAction.bind(null, orgID)}
      />
    </div>
  );
}
