import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { requireAgentsFeature } from "@/common/lib/growthbook/require-agents-feature";
import { translatedTitleHead } from "@/common/lib/document-head";
import { AutomationDetailSkeleton } from "@/features/agent-automations/components/automation-detail-skeleton";
import { AutomationDetailPage } from "@/features/agent-automations/pages/automation-detail-page";

export const Route = createFileRoute("/automations_/$automationId")({
  staticData: { chrome: true },
  component: AutomationDetailRoute,
  pendingComponent: AutomationDetailSkeleton,
  beforeLoad: ({ context, location }) => {
    requireAuth()({ context, location });
    return requireAgentsFeature()({ context });
  },
  head: ({ match }) =>
    translatedTitleHead("agentAutomations.detailTitle", match),
});

function AutomationDetailRoute() {
  const { automationId } = Route.useParams();
  return <AutomationDetailPage automationId={automationId} />;
}
