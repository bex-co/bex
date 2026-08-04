import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";

// t003 placeholder: the /agents list + composer link here, but the real detail
// page (metadata header, PR/evidence cards, the live conversation column, and
// steering) is t004's scope. This bare route exists so the typed links from the
// list rows and the composer's post-create navigation resolve and typecheck;
// t004 replaces this component wholesale.
export const Route = createFileRoute("/agents/$agentSessionId")({
  component: AgentSessionDetailPage,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("agentSessions.pageTitle", match),
});

function AgentSessionDetailPage() {
  const { t } = useTranslations();
  const { agentSessionId } = Route.useParams();

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl py-10 text-center">
          <p className="font-medium">
            {t("agentSessions.detailPlaceholderTitle")}
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("agentSessions.detailPlaceholderBody", { id: agentSessionId })}
          </p>
        </div>
      </div>
    </DashboardLayout>
  );
}
