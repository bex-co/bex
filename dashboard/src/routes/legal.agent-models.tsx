import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { LegalAgentModelsRouteSkeleton } from "@/common/components/route-skeletons";
import AgentModelTermsPage from "@/features/legal/pages/agent-model-terms-page";

// Public: no requireAuth and no chrome, so it reads the same signed in or out.
export const Route = createFileRoute("/legal/agent-models")({
  component: AgentModelTermsPage,
  pendingComponent: LegalAgentModelsRouteSkeleton,
  head: ({ match }) => translatedTitleHead("legal.agentModels.title", match),
});
