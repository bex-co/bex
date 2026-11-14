import { Link } from "@tanstack/react-router";
import { GitPullRequest } from "lucide-react";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import {
  AGENT_SESSION_PR_STATES,
  type AgentSessionListSearch,
} from "@/features/agent-sessions/types";

/** Any, then each PR state; `undefined` drops the filter. */
const OPTIONS = [
  { value: undefined, labelKey: "agentSessions.prStateFilterAny" },
  ...AGENT_SESSION_PR_STATES.map((value) => ({
    value,
    labelKey: `agentSessions.prState.${value}`,
  })),
] as const;

/** The list heading row the filter sits in, shared with the page skeleton. */
export const SESSION_LIST_HEADING_ROW_CLASS =
  "flex h-7 items-center justify-between gap-2";

// Shared by the control and its skeleton so the two keep one size: the
// skeleton draws the same box around the same labels, made invisible.
const GROUP_CLASS = "flex h-7 shrink-0 items-center rounded-md p-0.5";
const ICON_CLASS = "mx-1 size-3.5 shrink-0";
const OPTION_CLASS =
  "flex h-6 items-center rounded px-2 text-xs font-medium whitespace-nowrap";

/**
 * The session list's PR-state filter (Devin's `pr_state`): Any, Open,
 * Merged or Closed, one at a time. Each option is a link that keeps the
 * list's other filters and sets or drops `?prState=`, so a filtered list can
 * be shared; with no other filter, Any returns to the default /agents page.
 */
export function SessionPrStateFilter({
  search,
}: {
  /** The list's current filters, kept when the PR state changes. */
  search: AgentSessionListSearch;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="group"
      aria-label={t("agentSessions.prStateFilter")}
      className={cn(GROUP_CLASS, "bg-muted")}
    >
      <GitPullRequest
        aria-hidden
        className={cn(ICON_CLASS, "text-muted-foreground")}
      />
      {OPTIONS.map(({ value, labelKey }) => {
        const active = search.prState === value;
        return (
          <Link
            key={value ?? "any"}
            to="/agents"
            search={{ ...search, prState: value }}
            aria-current={active ? "true" : undefined}
            className={cn(
              OPTION_CLASS,
              active
                ? "bg-background text-foreground shadow-xs"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {t(labelKey)}
          </Link>
        );
      })}
    </div>
  );
}

/** The filter's pending shape: its own box and labels, drawn invisibly. */
export function SessionPrStateFilterSkeleton() {
  const { t } = useTranslations();
  return (
    <Skeleton
      className={cn(GROUP_CLASS, "text-transparent")}
      data-skeleton-region="pr-state-filter"
    >
      <GitPullRequest aria-hidden className={ICON_CLASS} />
      {OPTIONS.map(({ value, labelKey }) => (
        <span key={value ?? "any"} className={OPTION_CLASS}>
          {t(labelKey)}
        </span>
      ))}
    </Skeleton>
  );
}
