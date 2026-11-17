import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { requireAgentsFeature } from "@/common/lib/growthbook/require-agents-feature";
import { translatedTitleHead } from "@/common/lib/document-head";
import { AutomationEditSkeleton } from "@/features/agent-automations/components/automation-form-skeleton";
import { AutomationEditPage } from "@/features/agent-automations/pages/automation-form-page";

export const Route = createFileRoute("/automations_/$automationId_/edit")({
  staticData: { chrome: true },
  component: AutomationEditRoute,
  pendingComponent: AutomationEditSkeleton,
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
  head: ({ match }) => translatedTitleHead("agentAutomations.editTitle", match),
});

function AutomationEditRoute() {
  const { automationId } = Route.useParams();
  return <AutomationEditPage automationId={automationId} />;
}
