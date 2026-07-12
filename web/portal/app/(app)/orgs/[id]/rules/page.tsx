import { requireAdmin } from "@/lib/guards";
import { listRules } from "@/lib/portal-api";
import { RulesManager } from "@/components/admin/rules-manager";
import { createRuleAction, deleteRuleAction, setRuleEnabledAction } from "@/lib/admin-actions";

export default async function RulesPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const orgID = Number(id);
  await requireAdmin(orgID);
  const rules = await listRules(orgID);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Auto-acceptance rules</h1>
        <p className="mt-1 text-sm text-ink-muted">
          Approve license requests automatically when they match a rule. Anything that doesn&apos;t
          match is reviewed in the request queue.
        </p>
      </div>
      <RulesManager
        rules={rules}
        create={createRuleAction.bind(null, orgID)}
        setEnabled={setRuleEnabledAction.bind(null, orgID)}
        remove={deleteRuleAction.bind(null, orgID)}
      />
    </div>
  );
}
