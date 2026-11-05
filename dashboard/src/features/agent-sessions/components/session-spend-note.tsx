import { useTranslations } from "@/common/hooks/use-translations";
import { formatPreciseUsd } from "@/features/agent-sessions/lib/spend-limit";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/**
 * The session's exact model spend (`outcome.cost.modelUSD`, ADR094 D7) beside
 * its `maxSpendUSD` ceiling when one was set. Absent when the session made no
 * priced request (a workspace's own model key, or nothing run yet), so it adds
 * no region to the loading skeleton.
 */
export function SessionSpendNote({ session }: { session: AgentSessionView }) {
  const { t, i18n } = useTranslations();
  const cost = session.outcome?.cost;
  if (!cost) return null;
  const spent = formatPreciseUsd(cost.modelUSD, i18n.language);
  return (
    <p
      className="text-muted-foreground px-1 text-xs"
      data-testid="agent-session-spend-note"
    >
      {session.maxSpendUSD
        ? t("agentSessions.spendNoteWithLimit", {
            spent,
            limit: formatPreciseUsd(session.maxSpendUSD, i18n.language),
          })
        : t("agentSessions.spendNote", { spent })}
    </p>
  );
}
