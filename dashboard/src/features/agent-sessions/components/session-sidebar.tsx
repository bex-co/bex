import { Link } from "@tanstack/react-router";
import { GitPullRequest, Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessions } from "@/features/agent-sessions/hooks/use-agent-sessions";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { AgentSessionPhaseChip } from "@/features/agent-sessions/components/session-list";

export interface SessionSidebarProps {
  /** The session currently open in the chat view — highlighted in the list. */
  activeId: string;
}

/**
 * The left sessions rail on the full-page chat view (t007): a "New session"
 * affordance (routes to `/agents`, which keeps the standalone list + composer)
 * plus a "Recent" list keyed on `useAgentSessions` — each row a title + a
 * phase/status line + a PR ref. Clicking a row navigates to that session,
 * staying inside the chat view. Hidden below `lg` (the header's back link and
 * `/agents` remain the narrow-screen navigation).
 */
export function SessionSidebar({ activeId }: SessionSidebarProps) {
  const { t } = useTranslations();
  const { sessions, loading } = useAgentSessions();

  return (
    <aside
      aria-label={t("agentSessions.sidebarLabel")}
      className="bg-muted/20 hidden w-72 shrink-0 flex-col border-r lg:flex"
    >
      <div className="border-b p-3">
        <Button asChild size="sm" className="w-full justify-start">
          <Link to="/agents">
            <Plus className="size-4" />
            {t("agentSessions.newSession")}
          </Link>
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-2">
        <p className="text-muted-foreground px-2 py-1.5 text-xs font-medium tracking-wide uppercase">
          {t("agentSessions.recentSessions")}
        </p>

        {loading && sessions.length === 0 ? (
          <div className="space-y-1.5 p-1">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : sessions.length === 0 ? (
          <p className="text-muted-foreground px-2 py-3 text-sm">
            {t("agentSessions.sidebarEmpty")}
          </p>
        ) : (
          <ul className="space-y-0.5">
            {sessions.map((s) => (
              <li key={s.id}>
                <SessionRow session={s} active={s.id === activeId} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </aside>
  );
}

function SessionRow({
  session,
  active,
}: {
  session: AgentSessionView;
  active: boolean;
}) {
  const title = session.agentConfig.task || session.id;
  return (
    <Link
      to="/agents/$agentSessionId"
      params={{ agentSessionId: session.id }}
      className={cn(
        "hover:bg-muted flex flex-col gap-1 rounded-lg px-2 py-2 transition-colors",
        active && "bg-muted",
      )}
    >
      <span
        className="text-foreground truncate text-sm font-medium"
        title={title}
      >
        {title}
      </span>
      <span className="flex items-center gap-1.5">
        <AgentSessionPhaseChip phase={session.phase} />
        {session.prNumber != null ? (
          <span className="text-muted-foreground inline-flex items-center gap-0.5 text-xs">
            <GitPullRequest className="size-3" />#{session.prNumber}
          </span>
        ) : null}
      </span>
    </Link>
  );
}
