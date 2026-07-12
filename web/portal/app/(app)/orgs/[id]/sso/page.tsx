import { requireAdmin } from "@/lib/guards";
import { getSSO } from "@/lib/portal-api";
import { SSOPanel } from "@/components/admin/sso-panel";
import { setupSSOAction } from "@/lib/admin-actions";

export default async function SSOPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const orgID = Number(id);
  await requireAdmin(orgID);
  const config = await getSSO(orgID);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Single sign-on</h1>
        <p className="mt-1 text-sm text-ink-muted">
          Federate your identity provider so members sign in with your organization&apos;s accounts.
        </p>
      </div>
      <SSOPanel config={config} setup={setupSSOAction.bind(null, orgID)} />
    </div>
  );
}
