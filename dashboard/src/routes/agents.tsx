import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessions } from "@/features/agent-sessions/hooks/use-agent-sessions";
import { NewSessionComposer } from "@/features/agent-sessions/components/new-session-composer";
import { SessionList } from "@/features/agent-sessions/components/session-list";

export const Route = createFileRoute("/agents")({
  staticData: { chrome: true },
  component: AgentSessionsPage,
  beforeLoad: requireAuth(),
  // `?view=list` keeps the standalone sessions table reachable (the sidebar's
  // More/view-all target) without a second route file.
  validateSearch: (search: Record<string, unknown>): { view?: "list" } =>
    search.view === "list" ? { view: "list" } : {},
  head: ({ match }) => translatedTitleHead("agentSessions.pageTitle", match),
});

/**
 * The `/agents` page (ADR047 D9): a main pane that is either the Devin-style
 * centered prompt box or, under `?view=list`, the standalone sessions table.
 * The sessions list lives in the ONE dashboard rail (`AgentSessionsNavSection`,
 * w5/m64) — this page renders no sidebar of its own. Workspace scoping comes
 * from the switcher (`useAgentSessions` reads `useWorkspace()`), never the
 * path. The composer always renders — a 503/unconfigured backend shows its
 * house callout on the composer while the rail's list degrades on its own.
 */
export function AgentSessionsPage() {
  const { view } = Route.useSearch();

  return (
    <DashboardLayout>
      <div className="flex min-w-0 min-h-0 flex-1 flex-col">
        {view === "list" ? <SessionListPane /> : <ComposerPane />}
      </div>
    </DashboardLayout>
  );
}

function ComposerPane() {
  const { t } = useTranslations();
  return (
    <div className="flex flex-1 items-center justify-center overflow-auto p-4 sm:p-6">
      <div className="w-full max-w-2xl space-y-4 pb-16">
        <div className="space-y-1.5 text-center">
          <h1 className="text-xl font-semibold">
            {t("agentSessions.promptHeading")}
          </h1>
          <p className="text-muted-foreground text-sm">
            {t("agentSessions.pageSubtitle")}
          </p>
        </div>
        <NewSessionComposer />
      </div>
    </div>
  );
}

function SessionListPane() {
  const { t } = useTranslations();
  // The rail's AgentSessionsNavSection renders alongside and owns the poll;
  // this is a cache read, so only one poller is ever live on /agents.
  const { sessions, loading, error } = useAgentSessions({ poll: false });
  return (
    <>
      <div className="border-b px-4 py-3 sm:px-6">
        <h1 className="text-xl font-semibold">
          {t("agentSessions.pageTitle")}
        </h1>
        <p className="text-muted-foreground mt-1 text-sm">
          {t("agentSessions.pageSubtitle")}
        </p>
      </div>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl">
          <SessionList sessions={sessions} loading={loading} error={error} />
        </div>
      </div>
    </>
  );
}
