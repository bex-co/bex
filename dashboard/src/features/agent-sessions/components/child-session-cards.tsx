import { Link } from "@tanstack/react-router";
import { GitBranch } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { AgentStatusBadge } from "@/features/agent-sessions/components/agent-status-badge";
import { sessionTitleShort } from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/**
 * One card per session the agent spawned (ADR102), in creation order, inside
 * the parent's transcript footer. The transcript carries no event for a spawn,
 * so the cards sit in a labelled "Child sessions" block; each links to the
 * child. Renders nothing for a session without children, so it adds no region
 * to the loading skeleton.
 */
export function ChildSessionCards({
  childSessions,
}: {
  childSessions: readonly AgentSessionView[];
}) {
  const { t } = useTranslations();
  if (childSessions.length === 0) return null;
  return (
    <section
      aria-label={t("agentSessions.lineage.childSessions")}
      data-testid="child-session-cards"
      className="space-y-1.5"
    >
      <h2 className="text-muted-foreground text-xs font-medium">
        {t("agentSessions.lineage.childSessions")}
      </h2>
      <ul className="space-y-1.5">
        {childSessions.map((child) => (
          <li key={child.id}>
            <Link
              to="/agents/$agentSessionId"
              params={{ agentSessionId: child.id }}
              className="bg-card hover:bg-accent flex min-w-0 items-center gap-3 rounded-lg border px-3 py-2 text-sm"
            >
              <GitBranch
                aria-hidden
                className="text-muted-foreground size-4 shrink-0"
              />
              <span className="min-w-0 flex-1">
                <span className="text-muted-foreground block text-xs">
                  {t("agentSessions.lineage.childCardTitle")}
                </span>
                <span className="block truncate font-medium">
                  {sessionTitleShort(child)}
                </span>
              </span>
              <AgentStatusBadge session={child} />
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
