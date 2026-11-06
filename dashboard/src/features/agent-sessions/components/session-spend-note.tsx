import { useTranslations } from "@/common/hooks/use-translations";
import { formatPreciseUsd } from "@/features/agent-sessions/lib/spend-limit";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/**
 * What the session has cost (`outcome.cost`, ADR094 D7): the total, as Devin
 * shows one consumption figure, with the model and sandbox compute parts
 * beside it (w9/m178). `maxSpendUSD` caps model spend only, so the limit sits
 * next to the model part. A server without `totalUSD` gets the model-only
 * line. Absent when nothing is priced yet, so it adds no region to the
 * loading skeleton.
 */
export function SessionSpendNote({ session }: { session: AgentSessionView }) {
  const { t, i18n } = useTranslations();
  const cost = session.outcome?.cost;
  if (!cost) return null;
  const usd = (value: string) => formatPreciseUsd(value, i18n.language);
  const model = usd(cost.modelUSD);
  const limit = session.maxSpendUSD ? usd(session.maxSpendUSD) : null;
  let text: string;
  if (cost.totalUSD != null) {
    const parts = {
      total: usd(cost.totalUSD),
      model,
      compute: usd(cost.computeUSD ?? "0"),
    };
    text = limit
      ? t("agentSessions.costNoteWithLimit", { ...parts, limit })
      : t("agentSessions.costNote", parts);
  } else {
    text = limit
      ? t("agentSessions.spendNoteWithLimit", { spent: model, limit })
      : t("agentSessions.spendNote", { spent: model });
  }
  return (
    <p
      className="text-muted-foreground px-1 text-xs"
      data-testid="agent-session-spend-note"
    >
      {text}
    </p>
  );
}
