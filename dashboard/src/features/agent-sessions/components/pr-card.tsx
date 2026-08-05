import { ExternalLink, GitPullRequest } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/** Short 7-char SHA, matching the git/GitHub convention. */
function shortSha(sha: string | null): string | null {
  return sha ? sha.slice(0, 7) : null;
}

export interface PrCardProps {
  session: AgentSessionView;
}

/**
 * The draft-PR card rendered INLINE in the conversation flow (ADR047 D8, t005):
 * Devin-style — the PR title (the session's task intent), a `<repo>#<number> ·
 * <commits> · bot` reference line, and a review/open action. It renders only
 * once the agent has opened the PR (`prNumber != null`); the route places it at
 * the foot of the transcript so it reads as the turn's delivery.
 *
 * Diff stats (+added/−deleted) are NOT exposed by the current GraphQL surface —
 * the card shows the commit count from the bounded evidence when available and
 * omits the +/− stat (filed as a follow-up when the API adds it).
 */
export function PrCard({ session }: PrCardProps) {
  const { t } = useTranslations();
  if (session.prNumber == null) return null;

  const sha = shortSha(session.headSha);
  const title = session.agentConfig.task || t("agentSessions.prInlineTitle");
  const ref = `${session.repo}#${session.prNumber}`;
  const commits = session.evidence?.commits ?? null;

  return (
    <div className="border-border/70 bg-muted/10 my-3 rounded-xl border p-4">
      <div className="flex items-start gap-3">
        <div className="border-border/60 bg-background flex size-8 shrink-0 items-center justify-center rounded-full border">
          <GitPullRequest className="text-primary size-4" />
        </div>
        <div className="min-w-0 flex-1 space-y-1">
          <p className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
            {t("agentSessions.prInlineTitle")}
          </p>
          <p
            className="text-foreground truncate text-sm font-medium"
            title={title}
          >
            {title}
          </p>
          <div className="text-muted-foreground flex flex-wrap items-center gap-x-1.5 gap-y-1 text-xs">
            <code className="bg-muted rounded px-1.5 py-0.5 font-mono">
              {ref}
            </code>
            {commits != null ? (
              <>
                <span aria-hidden>·</span>
                <span>
                  {t("agentSessions.evidenceCommits", { count: commits })}
                </span>
              </>
            ) : null}
            {sha ? (
              <>
                <span aria-hidden>·</span>
                <code className="font-mono">{sha}</code>
              </>
            ) : null}
            <span aria-hidden>·</span>
            <Badge
              variant="secondary"
              className="px-1.5 py-0 font-mono text-[10px]"
            >
              {t("agentSessions.prBot")}
            </Badge>
          </div>
        </div>
        {session.prUrl ? (
          <Button asChild size="sm" variant="outline" className="shrink-0">
            <a href={session.prUrl} target="_blank" rel="noreferrer">
              {t("agentSessions.prReview")}
              <ExternalLink className="size-3.5" />
            </a>
          </Button>
        ) : null}
      </div>
    </div>
  );
}
