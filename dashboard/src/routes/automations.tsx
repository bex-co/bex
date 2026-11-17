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
  // The skeleton is drawn with the page's own components, so leaving it in
  // the route tree (the vite.config.ts default) would load the page up front
  // for everyone: it stays in the page's chunk.
  codeSplitGroupings: [
    ["component", "pendingComponent"],
    ["errorComponent"],
    ["notFoundComponent"],
  ],
  beforeLoad: ({ context, location }) => {
    requireAuth()({ context, location });
    return requireAgentsFeature()({ context });
  },
  head: ({ match }) => translatedTitleHead("agentAutomations.pageTitle", match),
});
