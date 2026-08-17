import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
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
import { AGENT_SESSION_PHASES } from "@/features/agent-sessions/types";
import type { AgentSessionPhase } from "@/features/agent-sessions/types";

/** Archive-membership values the `?archived=` search param accepts (ADR065). */
type ArchivedParam = "true" | "all";

export interface AgentsSearch {
  view?: "list";
  archived?: ArchivedParam;
}

export const Route = createFileRoute("/agents")({
  staticData: { chrome: true },
  component: AgentSessionsPage,
  beforeLoad: requireAuth(),
  // `?view=list` keeps the standalone sessions table reachable (the sidebar's
  // More/view-all target) without a second route file; `?archived=true|all`
  // widens it past the default unarchived working set (ADR065 D3 — the
  // sidebar's Archived entry targets `?view=list&archived=true`).
  validateSearch: (search: Record<string, unknown>): AgentsSearch => {
    const out: AgentsSearch = {};
    if (search.view === "list") out.view = "list";
    if (search.archived === "true" || search.archived === "all") {
      out.archived = search.archived;
    }
    return out;
  },
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
function AgentSessionsPage() {
  const { view, archived } = Route.useSearch();

  return (
    <DashboardLayout>
      <div className="flex min-w-0 min-h-0 flex-1 flex-col">
        {view === "list" ? (
          <SessionListPane archived={archived} />
        ) : (
          <ComposerPane />
        )}
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

function SessionListPane({ archived }: { archived?: ArchivedParam }) {
  const { t } = useTranslations();
  const [phase, setPhase] = useState<AgentSessionPhase | "all">("all");
  // The rail's AgentSessionsNavSection renders alongside and owns the poll for
  // the default working-set variables; every widened/filtered read here is its
  // own cache entry, refreshed by `refetch` after row mutations — never polled.
  const {
    sessions,
    loading,
    error,
    refetch,
    loadMore,
    loadingMore,
    hasMore,
  } = useAgentSessions({
    poll: false,
    archived,
    phases: phase === "all" ? undefined : [phase],
  });

  const membershipTabs: Array<{
    key: string;
    label: string;
    archived?: ArchivedParam;
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
        <div className="mx-auto w-full max-w-4xl space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-1" role="group">
              {membershipTabs.map((tab) => (
                <Button
                  key={tab.key}
                  asChild
                  size="sm"
                  variant={activeKey === tab.key ? "secondary" : "ghost"}
                >
                  <Link
                    to="/agents"
                    search={{ view: "list", archived: tab.archived }}
                  >
                    {tab.label}
                  </Link>
                </Button>
              ))}
            </div>
            <Select
              value={phase}
              onValueChange={(v) => setPhase(v as AgentSessionPhase | "all")}
            >
              <SelectTrigger
                size="sm"
                className="w-44"
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
          <SessionList
            sessions={sessions}
            loading={loading}
            error={error}
            onChanged={() => void refetch()}
          />
          {hasMore ? (
            <div className="flex justify-center">
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
        </div>
      </div>
    </>
  );
}
