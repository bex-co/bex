import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { AgentsPageSkeleton } from "@/common/components/detail-skeletons";
import { Loader2 } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { requireAgentsFeature } from "@/common/lib/growthbook/require-agents-feature";
import {
  translatedTitleHead,
  titleLoaderFetchPolicy,
} from "@/common/lib/document-head";
import { prefetchInParallel } from "@/common/lib/prefetch";
import { AgentSessionsDocument } from "@/graphql/definitions";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  AGENT_SESSION_PAGE_SIZE,
  useAgentSessions,
} from "@/features/agent-sessions/hooks/use-agent-sessions";
import { NewSessionComposer } from "@/features/agent-sessions/components/new-session-composer";
import { SessionList } from "@/features/agent-sessions/components/session-list";
import {
  SESSION_LIST_HEADING_ROW_CLASS,
  SessionPrStateFilter,
} from "@/features/agent-sessions/components/session-pr-state-filter";
import {
  agentSessionArchivedQueryValue,
  parseAgentSessionArchivedFilter,
  parseAgentSessionPhase,
  parseAgentSessionPrState,
  type AgentSessionListSearch,
} from "@/features/agent-sessions/types";

export interface AgentsSearch extends AgentSessionListSearch {
  /** Legacy pane selector; accepted so saved `?view=list` URLs keep working. */
  view?: "list";
}

export const Route = createFileRoute("/agents")({
  staticData: { chrome: true },
  component: AgentSessionsPage,
  pendingComponent: AgentSessionsPendingPage,
  beforeLoad: ({ context, location }) => {
    requireAuth()({ context, location });
    return requireAgentsFeature()({ context });
  },
  // Prefetch the requested working set on hover-intent so `/agents` mounts warm
  // (same pattern as `/` and `/blueprints`). Variables match the list hook's
  // archive/phase/tag/PR-state filters and default page size exactly.
  loaderDeps: ({ search }) => ({
    archived: search.archived,
    phase: search.phase,
    tag: search.tag,
    prState: search.prState,
  }),
  loader: ({ context, cause, deps }) => {
    const ownerId = context.workspaceId;
    if (ownerId == null) return;
    return prefetchInParallel([
      () =>
        context.client.query({
          query: AgentSessionsDocument,
          variables: {
            ownerId,
            archived: agentSessionArchivedQueryValue(deps.archived),
            phases: deps.phase ? [deps.phase] : null,
            repo: null,
            tag: deps.tag ?? null,
            mine: null,
            prStates: deps.prState ? [deps.prState] : null,
            limit: AGENT_SESSION_PAGE_SIZE,
          },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
    ]);
  },
  // `?view=list` is a compatibility no-op now that creation and history share
  // one page. `?archived=archived|all` widens the default unarchived working set
  // (ADR065 D3 — the sidebar's Archived entry targets `?archived=archived`);
  // legacy `?archived=true` is still accepted for old links and maps to "archived".
  // `?phase=` is still honored if linked; the page no longer offers a dropdown.
  // `?prState=open|merged|closed` is the list's PR-state filter.
  // A rejected value is undefined, not omitted: the router merges this over the
  // raw search, so `?phase=bogus` would otherwise reach the list query (w5/100).
  validateSearch: (search: Record<string, unknown>): AgentsSearch => ({
    view: search.view === "list" ? "list" : undefined,
    archived: parseAgentSessionArchivedFilter(search.archived),
    phase: parseAgentSessionPhase(search.phase),
    tag: typeof search.tag === "string" && search.tag ? search.tag : undefined,
    prState: parseAgentSessionPrState(search.prState),
  }),
  head: ({ match }) => translatedTitleHead("agentSessions.pageTitle", match),
});

/**
 * The `/agents` page (ADR047 D9): a prompt workspace. The unarchived working
 * set lives only in the ONE dashboard rail (`AgentSessionsNavSection`,
 * w5/m64); explicit Archived/All/phase URLs swap the composer for the wider
 * history view. This page renders no sidebar of its own.
 */
function AgentSessionsPage() {
  const { archived, phase, tag, prState } = Route.useSearch();
  return (
    <AgentSessionsPageContent
      archived={archived}
      phase={phase}
      tag={tag}
      prState={prState}
    />
  );
}

function AgentSessionsPendingPage() {
  const search = Route.useSearch();
  return <AgentsPageSkeleton mode={agentSessionsPageMode(search)} />;
}

function agentSessionsPageMode({
  archived,
  phase,
  tag,
  prState,
}: AgentSessionListSearch): "composer" | "list" {
  return archived != null || phase != null || tag != null || prState != null
    ? "list"
    : "composer";
}

export function AgentSessionsPageContent({
  archived,
  phase,
  tag,
  prState,
}: AgentSessionListSearch) {
  const listMode =
    agentSessionsPageMode({ archived, phase, tag, prState }) === "list";

  return (
    <DashboardLayout>
      <div className="min-h-0 min-w-0 flex-1 overflow-auto">
        <div className="mx-auto w-full max-w-5xl space-y-8 p-4 sm:p-6">
          {listMode ? (
            <>
              <PageHeader />
              <SessionListSection
                archived={archived}
                phase={phase}
                tag={tag}
                prState={prState}
              />
            </>
          ) : (
            <ComposerSection />
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

function PageHeader() {
  const { t } = useTranslations();
  return (
    <header className="mx-auto w-full max-w-4xl">
      <h1 className="text-xl font-semibold">{t("agentSessions.pageTitle")}</h1>
    </header>
  );
}

function ComposerSection() {
  const { t } = useTranslations();
  return (
    <section
      className="mx-auto w-full max-w-[40rem] space-y-3"
      aria-label={t("agentSessions.taskLabel")}
    >
      {/* The default page has no visible title; give assistive technology the
          one h1 list mode shows (w4/m249). */}
      <h1 className="sr-only">{t("agentSessions.pageTitle")}</h1>
      <NewSessionComposer />
    </section>
  );
}

function SessionListSection({
  archived,
  phase,
  tag,
  prState,
}: AgentSessionListSearch) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { sessions, loading, error, refetch, loadMore, loadingMore, hasMore } =
    useAgentSessions({
      poll: false,
      archived,
      phases: phase ? [phase] : undefined,
      tag,
      prStates: prState ? [prState] : undefined,
    });

  const heading = tag
    ? t("agentSessions.filterTagged", { tag })
    : archived === "archived"
      ? t("agentSessions.filterArchived")
      : archived === "all"
        ? t("agentSessions.filterAll")
        : t("agentSessions.filterActive");

  return (
    <section
      className="mx-auto w-full max-w-4xl space-y-3"
      aria-labelledby="session-list-title"
    >
      {/* One row at every width: the heading truncates rather than wrapping
          the filter onto a second line, so the pending skeleton (which draws
          this row at the same height) never shifts on load. */}
      <div className={SESSION_LIST_HEADING_ROW_CLASS}>
        <h2
          id="session-list-title"
          className="min-w-0 truncate text-base font-semibold"
        >
          {heading}
        </h2>
        <SessionPrStateFilter search={{ archived, phase, tag, prState }} />
      </div>
      <SessionList
        sessions={sessions}
        loading={loading}
        error={error}
        archiveFilter={archived}
        phase={phase}
        tag={tag}
        prState={prState}
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
