import { GitPullRequest, MessageSquare, XCircle } from "lucide-react";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import { safeHttpHref } from "@/common/lib/external-url";
import type { TurnOrigin } from "@/features/agent-sessions/hooks/use-agent-session-turn-origins";

const ICONS = {
  pr_review: GitPullRequest,
  pr_comment: MessageSquare,
  ci_failure: XCircle,
} as const;

// The line for one origin, or null for a kind this build does not know: a
// wrong claim about why a turn started is worse than none. A turn-1 comment
// is the `/bex` comment that started the session (w9/m348), not a steer.
function originLabel(t: Translate, { kind, author, turn }: TurnOrigin) {
  if (turn === 1 && kind === "pr_comment") {
    return author
      ? t("agentSessions.turnOriginStartComment", { author })
      : t("agentSessions.turnOriginStartCommentAnonymous");
  }
  switch (kind) {
    case "pr_review":
      return author
        ? t("agentSessions.turnOriginReview", { author })
        : t("agentSessions.turnOriginReviewAnonymous");
    case "pr_comment":
      return author
        ? t("agentSessions.turnOriginComment", { author })
        : t("agentSessions.turnOriginCommentAnonymous");
    case "ci_failure":
      return t("agentSessions.turnOriginCiFailure");
    default:
      return null;
  }
}

/**
 * Why a steering turn started when PR feedback started it (w9/m344), or the
 * `/bex` comment that started the session on turn 1 (w9/m348): one muted,
 * right-aligned line per origin above the turn, each linking the comment,
 * review or failed run on GitHub. Only an http(s) URL becomes a link.
 */
export function TurnOrigins({ origins }: { origins: TurnOrigin[] }) {
  const { t } = useTranslations();
  const lines = origins.flatMap((origin) => {
    const label = originLabel(t, origin);
    return label ? [{ origin, label }] : [];
  });
  if (lines.length === 0) return null;
  const starting = lines.every(({ origin }) => origin.turn === 1);
  return (
    <ul
      className="text-muted-foreground mb-1 flex flex-col items-end gap-0.5 pl-8 text-xs"
      aria-label={t(
        starting
          ? "agentSessions.turnOriginsStart"
          : "agentSessions.turnOrigins",
      )}
      data-testid="turn-origins"
    >
      {lines.map(({ origin, label }, index) => {
        const Icon = ICONS[origin.kind as keyof typeof ICONS];
        const href = safeHttpHref(origin.url);
        return (
          <li
            key={`${origin.kind}-${origin.url ?? ""}-${index}`}
            className="flex max-w-full min-w-0 items-center gap-1"
          >
            <Icon aria-hidden className="size-3 shrink-0" />
            {href ? (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="hover:text-foreground truncate underline-offset-2 hover:underline"
              >
                {label}
              </a>
            ) : (
              <span className="truncate">{label}</span>
            )}
          </li>
        );
      })}
    </ul>
  );
}
