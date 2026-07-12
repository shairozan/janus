import { requireAdmin } from "@/lib/guards";
import { listActivity } from "@/lib/portal-api";
import { ActivityLog } from "@/components/admin/activity-log";

export default async function ActivityPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const orgID = Number(id);
  await requireAdmin(orgID);
  const events = await listActivity(orgID);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Activity</h1>
        <p className="mt-1 text-sm text-ink-muted">
          An append-only record of every action taken in your organization&apos;s portal.
        </p>
      </div>
      <ActivityLog events={events} />
    </div>
  );
}
