import { requireAdmin } from "@/lib/guards";
import { listProposals } from "@/lib/portal-api";
import { BillingScreen } from "@/components/onboarding/billing-screen";
import {
  acceptProposalAction,
  counterProposalAction,
  payProposalAction,
  requestProposalAction,
} from "@/lib/proposal-actions";

export default async function BillingPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const orgID = Number(id);
  await requireAdmin(orgID);
  const proposals = await listProposals(orgID);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Agreements &amp; billing</h1>
        <p className="mt-1 text-sm text-ink-muted">
          Request an agreement, review Janus&apos;s offer, and pay to activate it. Pricing comes from
          the offer — nothing is charged until you pay.
        </p>
      </div>
      <BillingScreen
        proposals={proposals}
        request={requestProposalAction.bind(null, orgID)}
        counter={counterProposalAction.bind(null, orgID)}
        accept={acceptProposalAction.bind(null, orgID)}
        pay={payProposalAction.bind(null, orgID)}
      />
    </div>
  );
}
