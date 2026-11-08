import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { requireAgentsFeature } from "@/common/lib/growthbook/require-agents-feature";
import { translatedTitleHead } from "@/common/lib/document-head";
import { AutomationsListSkeleton } from "@/features/agent-automations/components/automations-list-skeleton";
import { AutomationsPage } from "@/features/agent-automations/pages/automations-page";

// Agent automations (ADR101 D9): scheduled sessions. Gated like /agents, since
// every fire starts an agent session.
export const Route = createFileRoute("/automations")({
  staticData: { chrome: true },
  component: AutomationsPage,
  pendingComponent: AutomationsListSkeleton,
  beforeLoad: ({ context, location }) => {
    requireAuth()({ context, location });
    return requireAgentsFeature()({ context });
  },
  head: ({ match }) => translatedTitleHead("agentAutomations.pageTitle", match),
});
