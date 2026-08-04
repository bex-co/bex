import { ExternalLink, GitPullRequest } from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
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
 * The draft-PR summary card (ADR047 D8 delivery): links the pull request by
 * `#number` to `prUrl` in a new tab and shows the head SHA. Until the agent
 * pushes and opens the PR it shows a quiet "no pull request yet" note, so the
 * card renders in every phase (the delivery is the session's whole point).
 */
export function PrCard({ session }: PrCardProps) {
  const { t } = useTranslations();
  const sha = shortSha(session.headSha);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <GitPullRequest className="size-4" />
          {t("agentSessions.prCardTitle")}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {session.prNumber != null ? (
          session.prUrl ? (
            <a
              href={session.prUrl}
              target="_blank"
              rel="noreferrer"
              className="text-primary inline-flex items-center gap-1.5 font-medium hover:underline"
            >
              {t("agentSessions.prBadge", { number: session.prNumber })}
              <ExternalLink className="size-3.5" />
            </a>
          ) : (
            <Badge variant="secondary" className="gap-1">
              <GitPullRequest className="size-3.5" />
              {t("agentSessions.prBadge", { number: session.prNumber })}
            </Badge>
          )
        ) : (
          <p className="text-muted-foreground">
            {t("agentSessions.prCardNone")}
          </p>
        )}

        {sha ? (
          <div className="text-muted-foreground flex items-center justify-between gap-2 text-xs">
            <span>{t("agentSessions.prCardHeadSha")}</span>
            <code className="bg-muted rounded px-1.5 py-0.5 font-mono">
              {sha}
            </code>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
