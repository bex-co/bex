import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessions } from "@/features/agent-sessions/hooks/use-agent-sessions";
import { NewSessionComposer } from "@/features/agent-sessions/components/new-session-composer";
import { SessionList } from "@/features/agent-sessions/components/session-list";

export const Route = createFileRoute("/agents")({
  component: AgentSessionsPage,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("agentSessions.pageTitle", match),
});

/**
 * The `/agents` page (ADR047 D9): the workspace's coding-agent sessions list
 * plus the new-session composer. Workspace scoping comes from the switcher
 * (`useAgentSessions` reads `useWorkspace()`), never the path. The composer
 * always renders — even when the list errors — so a 503/unconfigured backend
 * shows its house callout on the composer while the list degrades on its own.
 */
export function AgentSessionsPage() {
  const { t } = useTranslations();
  const { sessions, loading, error } = useAgentSessions();

  return (
    <DashboardLayout>
      <div className="border-b px-4 py-4 sm:px-6">
        <h1 className="text-xl font-semibold">
          {t("agentSessions.pageTitle")}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("agentSessions.pageSubtitle")}
        </p>
      </div>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <NewSessionComposer />
          <SessionList sessions={sessions} loading={loading} error={error} />
        </div>
      </div>
    </DashboardLayout>
  );
}
