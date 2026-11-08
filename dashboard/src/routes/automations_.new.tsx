import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { requireAgentsFeature } from "@/common/lib/growthbook/require-agents-feature";
import { translatedTitleHead } from "@/common/lib/document-head";
import { AutomationCreateSkeleton } from "@/features/agent-automations/components/automation-form-skeleton";
import { AutomationCreatePage } from "@/features/agent-automations/pages/automation-form-page";

// The trailing underscore escapes the /automations list route's component
// hierarchy while keeping the URL `/automations/new`: the list renders no
// <Outlet />, so nesting would leave it mounted (the webhooks_.new precedent).
export const Route = createFileRoute("/automations_/new")({
  staticData: { chrome: true },
  component: AutomationCreatePage,
  pendingComponent: AutomationCreateSkeleton,
  beforeLoad: ({ context, location }) => {
    requireAuth()({ context, location });
    return requireAgentsFeature()({ context });
  },
  head: ({ match }) =>
    translatedTitleHead("agentAutomations.createTitle", match),
});
