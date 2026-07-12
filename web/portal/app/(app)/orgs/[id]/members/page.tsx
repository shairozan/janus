import { requireAdmin } from "@/lib/guards";
import { listMembers } from "@/lib/portal-api";
import { MembersTable } from "@/components/admin/members-table";
import {
  demoteMemberAction,
  offboardMemberAction,
  promoteMemberAction,
} from "@/lib/admin-actions";

export default async function MembersPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const orgID = Number(id);
  const profile = await requireAdmin(orgID);
  const members = await listMembers(orgID);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Members</h1>
        <p className="mt-1 text-sm text-ink-muted">
          Promote members to admins, or offboard departing users (frees their seat).
        </p>
      </div>
      <MembersTable
        members={members}
        currentUserId={profile.id}
        promote={promoteMemberAction.bind(null, orgID)}
        demote={demoteMemberAction.bind(null, orgID)}
        offboard={offboardMemberAction.bind(null, orgID)}
      />
    </div>
  );
}
