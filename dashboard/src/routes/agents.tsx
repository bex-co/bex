import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { ListPageSkeleton } from "@/common/components/detail-skeletons";
import { Loader2 } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessions } from "@/features/agent-sessions/hooks/use-agent-sessions";
import { NewSessionComposer } from "@/features/agent-sessions/components/new-session-composer";
import { SessionList } from "@/features/agent-sessions/components/session-list";
import {
  AGENT_SESSION_PHASES,
  type AgentSessionArchivedFilter,
  type AgentSessionListSearch,
  type AgentSessionPhase,
} from "@/features/agent-sessions/types";

export interface AgentsSearch extends AgentSessionListSearch {
  /** Legacy pane selector; accepted so saved `?view=list` URLs keep working. */
  view?: "list";
}

export const Route = createFileRoute("/agents")({
  staticData: { chrome: true },
  component: AgentSessionsPage,
  pendingComponent: ListPageSkeleton,
  beforeLoad: requireAuth(),
  // `?view=list` is a compatibility no-op now that creation and history share
  // one page. `?archived=true|all` widens the default unarchived working set
  // (ADR065 D3 — the sidebar's Archived entry targets `?archived=true`).
  validateSearch: (search: Record<string, unknown>): AgentsSearch => {
    const out: AgentsSearch = {};
    if (search.view === "list") out.view = "list";
    if (search.archived === "true" || search.archived === "all") {
      out.archived = search.archived;
    }
    if (
      typeof search.phase === "string" &&
      AGENT_SESSION_PHASES.includes(search.phase as AgentSessionPhase)
    ) {
      out.phase = search.phase as AgentSessionPhase;
    }
    return out;
  },
  head: ({ match }) => translatedTitleHead("agentSessions.pageTitle", match),
});

/**
 * The `/agents` page (ADR047 D9): one workspace for creating a session and
 * browsing its history. The recent working set also lives in the ONE dashboard
 * rail (`AgentSessionsNavSection`, w5/m64) — this page renders no sidebar of its
 * own. Workspace scoping comes from the switcher (`useAgentSessions` reads
 * `useWorkspace()`), never the path. The composer always renders — a
 * 503/unconfigured backend shows its house callout while the list degrades on
 * its own.
 */
function AgentSessionsPage() {
  const { archived, phase } = Route.useSearch();

  return (
    <DashboardLayout>
      <div className="min-h-0 min-w-0 flex-1 overflow-auto">
        <div className="mx-auto w-full max-w-5xl space-y-8 p-4 sm:p-6">
          <PageHeader />
          <ComposerSection />
          <SessionListSection archived={archived} phase={phase} />
        </div>
      </div>
    </DashboardLayout>
  );
}

function PageHeader() {
  const { t } = useTranslations();
  return (
    <header className="space-y-1">
      <h1 className="text-xl font-semibold">{t("agentSessions.pageTitle")}</h1>
      <p className="text-muted-foreground max-w-2xl text-sm">
        {t("agentSessions.pageSubtitle")}
      </p>
    </header>
  );
}

function ComposerSection() {
  const { t } = useTranslations();
  return (
    <section
      className="max-w-3xl space-y-3"
      aria-labelledby="new-session-title"
    >
      <h2 id="new-session-title" className="text-sm font-medium">
        {t("agentSessions.promptHeading")}
      </h2>
      <NewSessionComposer />
    </section>
  );
}

function SessionListSection({
  archived,
  phase,
}: {
  archived?: AgentSessionArchivedFilter;
  phase?: AgentSessionPhase;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  // The rail's AgentSessionsNavSection renders alongside and owns the poll for
  // the default working-set variables; every widened/filtered read here is its
  // own cache entry, refreshed by `refetch` after row mutations — never polled.
  const { sessions, loading, error, refetch, loadMore, loadingMore, hasMore } =
    useAgentSessions({
      poll: false,
      archived,
      phases: phase ? [phase] : undefined,
    });

  const membershipTabs: Array<{
    key: string;
    label: string;
    archived?: AgentSessionArchivedFilter;
  }> = [
    { key: "active", label: t("agentSessions.filterActive") },
    {
      key: "archived",
      label: t("agentSessions.filterArchived"),
      archived: "true",
    },
    { key: "all", label: t("agentSessions.filterAll"), archived: "all" },
  ];
  const activeKey =
    archived === "true" ? "archived" : archived === "all" ? "all" : "active";

  return (
    <section className="space-y-3" aria-labelledby="session-list-title">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="session-list-title" className="text-base font-semibold">
          {t("agentSessions.listTitle")}
        </h2>
        <div className="flex flex-wrap items-center gap-2">
          <div
            className="bg-muted/50 flex items-center gap-0.5 rounded-lg p-0.5"
            role="group"
          >
            {membershipTabs.map((tab) => (
              <Button
                key={tab.key}
                asChild
                size="sm"
                variant={activeKey === tab.key ? "secondary" : "ghost"}
                className="h-7 px-2.5 shadow-none"
              >
                <Link to="/agents" search={{ archived: tab.archived, phase }}>
                  {tab.label}
                </Link>
              </Button>
            ))}
          </div>
          <Select
            value={phase ?? "all"}
            onValueChange={(value) =>
              void navigate({
                to: "/agents",
                search: {
                  archived,
                  phase:
                    value === "all" ? undefined : (value as AgentSessionPhase),
                },
              })
            }
          >
            <SelectTrigger
              size="sm"
              className="h-8 w-40"
              aria-label={t("agentSessions.filterPhase")}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                {t("agentSessions.filterPhaseAll")}
              </SelectItem>
              {AGENT_SESSION_PHASES.map((p) => (
                <SelectItem key={p} value={p}>
                  {t(`agentSessions.phase.${p}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <SessionList
        sessions={sessions}
        loading={loading}
        error={error}
        archiveFilter={archived}
        phase={phase}
        onChanged={() => refetch()}
        onRetry={() => void refetch()}
        onClearFilters={() => void navigate({ to: "/agents", search: {} })}
      />
      {hasMore ? (
        <div className="flex justify-center pt-1">
          <Button
            variant="outline"
            size="sm"
            disabled={loadingMore}
            onClick={() => void loadMore()}
          >
            {loadingMore ? (
              <>
                <Loader2 className="animate-spin" />
                {t("agentSessions.loadingMore")}
              </>
            ) : (
              t("agentSessions.loadMore")
            )}
          </Button>
        </div>
      ) : null}
    </section>
  );
}
