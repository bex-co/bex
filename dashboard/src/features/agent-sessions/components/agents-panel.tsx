import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { AgentStatusBadge } from "@/features/agent-sessions/components/agent-status-badge";
import { groupByAgentStatus } from "@/features/agent-sessions/lib/lineage";
import { sessionTitleShort } from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/** The fixed height of one panel row; the skeleton row reuses it. */
const ROW_CLASS = "flex h-14 min-w-0 items-center gap-2 rounded-md border px-2";

/**
 * The session page's Agents panel (ADR102, Devin's "Agents"): the current
 * session's descendants grouped by status, each row with its title, state
 * badge and a View session link. A right-hand column from `lg`, a stacked
 * section above the conversation below it; collapsible either way. The parent
 * renders it only for a session in a lineage (`showAgentsPanel`), so it never
 * appears in the loading skeleton of an ordinary session.
 *
 * `loading` is for a child session, whose lineage is known from its own
 * `parentSessionId` before the tree answers: bounded placeholder rows with the
 * same height as real rows keep the panel from resizing on load.
 */
export function AgentsPanel({
  descendants,
  parentSessionId,
  loading,
}: {
  descendants: readonly AgentSessionView[];
  parentSessionId: string | null;
  loading: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(true);
  const buckets = groupByAgentStatus(descendants);
  const ToggleIcon = open ? ChevronDown : ChevronRight;

  return (
    <aside
      aria-label={t("agentSessions.lineage.agentsPanel")}
      data-testid="agents-panel"
      className="bg-background flex shrink-0 flex-col border-b lg:order-last lg:min-h-0 lg:w-72 lg:border-b-0 lg:border-l"
    >
      <div className="flex h-10 shrink-0 items-center justify-between gap-2 px-3">
        <h2 className="text-sm font-medium">
          {t("agentSessions.lineage.agentsPanel")}
        </h2>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-7"
          aria-expanded={open}
          aria-label={
            open
              ? t("agentSessions.lineage.collapsePanel")
              : t("agentSessions.lineage.expandPanel")
          }
          onClick={() => setOpen((v) => !v)}
        >
          <ToggleIcon aria-hidden className="size-4" />
        </Button>
      </div>
      {open ? (
        <div
          data-testid="agents-panel-body"
          className="max-h-56 space-y-3 overflow-y-auto px-3 pb-3 lg:max-h-none lg:min-h-0 lg:flex-1"
        >
          {parentSessionId ? (
            <Link
              to="/agents/$agentSessionId"
              params={{ agentSessionId: parentSessionId }}
              className="text-muted-foreground hover:text-foreground block text-xs underline-offset-2 hover:underline"
            >
              {t("agentSessions.lineage.parentSession")}
            </Link>
          ) : null}
          {loading && descendants.length === 0 ? (
            <div
              aria-hidden="true"
              data-testid="agents-panel-skeleton"
              className="space-y-1.5"
            >
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-14 w-full rounded-md" />
            </div>
          ) : null}
          {buckets.map(({ group, sessions }) => (
            <section
              key={group}
              data-status-group={group}
              aria-label={t(`agentSessions.lineage.status.${group}`)}
              className="space-y-1.5"
            >
              <h3 className="text-muted-foreground text-xs font-medium">
                {t(`agentSessions.lineage.status.${group}`)} {sessions.length}
              </h3>
              <ul className="space-y-1.5">
                {sessions.map((s) => (
                  <li key={s.id} className={ROW_CLASS}>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">
                        {sessionTitleShort(s)}
                      </span>
                      <AgentStatusBadge session={s} />
                    </span>
                    <Link
                      to="/agents/$agentSessionId"
                      params={{ agentSessionId: s.id }}
                      className="text-primary shrink-0 text-xs underline-offset-2 hover:underline"
                    >
                      {t("agentSessions.lineage.viewSession")}
                    </Link>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      ) : null}
    </aside>
  );
}
