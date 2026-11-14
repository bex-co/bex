import { Coins } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { AgentModelSource } from "@/graphql/definitions";
import { sessionUsd } from "@/features/usage/lib/charges";
import { formatPreciseUsd } from "@/features/agent-sessions/lib/spend-limit";
import { useAgentSessionModelCost } from "@/features/agent-sessions/hooks/use-agent-session-model-cost";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/** The meta-row slot's width, shared by the skeleton and the placeholder. */
const SLOT_CLASS = "h-3 w-14";

/**
 * The session's model cost in the detail header's meta row (ADR091 D6): the
 * total of its bex-provided model requests, with each turn's cost and tokens
 * in a popover. The session's own model route labels it: a session on the
 * workspace's own key reads "Billed by your provider". Once the session
 * reports `outcome.cost.totalUSD` (w9/m178), the slot shows that total, model
 * plus sandbox compute, like the session page's spend line; the popover adds
 * the split. While the cost loads, or when it can't be read, the slot keeps its
 * width so the meta row never shifts.
 */
export function SessionModelCost({ session }: { session: AgentSessionView }) {
  const { t, i18n } = useTranslations();
  const locale = i18n.language;
  const { cost, loading } = useAgentSessionModelCost(
    session.id,
    session.turns,
    !session.isTerminal,
  );
  const number = new Intl.NumberFormat(locale);
  const amount = (costUsd: string, requests: number) =>
    sessionUsd(costUsd, requests, locale, (value) =>
      t("usage.lessThanAmount", { amount: value }),
    );

  if (loading) {
    return (
      <Skeleton
        className={SLOT_CLASS}
        data-testid="session-model-cost-skeleton"
      />
    );
  }
  if (!cost) {
    return (
      <span
        aria-hidden="true"
        className={`inline-block ${SLOT_CLASS}`}
        data-testid="session-model-cost-placeholder"
      />
    );
  }
  const total = session.outcome?.cost;
  const hasTotal = total?.totalUSD != null && Number(total.totalUSD) > 0;
  const precise = (value: string) => formatPreciseUsd(value, locale);
  const label = hasTotal
    ? t("agentSessions.costTotalAll", { amount: precise(total.totalUSD!) })
    : t("agentSessions.costTotal", {
        amount: amount(cost.costUsd, cost.requests),
      });
  if (cost.modelSource === AgentModelSource.Byo && !hasTotal) {
    return <span>{t("agentSessions.costByo")}</span>;
  }
  if (cost.requests === 0) {
    return <span data-testid="session-cost-label">{label}</span>;
  }
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="link"
          className="text-muted-foreground h-auto gap-1 p-0 text-xs font-normal"
        >
          <Coins className="size-3" />
          <span data-testid="session-cost-label">{label}</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-80 space-y-3 text-sm"
        aria-label={t("agentSessions.costTitle")}
      >
        <p className="font-medium">{t("agentSessions.costTitle")}</p>
        {hasTotal ? (
          <p className="text-muted-foreground text-xs">
            {t("agentSessions.costBreakdown", {
              model: precise(total.modelUSD),
              compute: precise(total.computeUSD ?? "0"),
            })}
          </p>
        ) : null}
        <ul className="space-y-2" aria-label={t("agentSessions.costTitle")}>
          {cost.turns.map((turn) => (
            <li key={turn.turn} className="space-y-0.5">
              <div className="flex items-baseline justify-between gap-3">
                <span>
                  {turn.turn > 0
                    ? t("agentSessions.costTurn", { turn: turn.turn })
                    : t("agentSessions.costSetup")}
                </span>
                <span className="font-mono tabular-nums">
                  {amount(turn.costUsd, turn.requests)}
                </span>
              </div>
              <p className="text-muted-foreground text-xs">
                {t("agentSessions.costTurnTokens", {
                  input: number.format(turn.inputTokens),
                  output: number.format(turn.outputTokens),
                  cache: number.format(
                    turn.cacheReadTokens + turn.cacheWriteTokens,
                  ),
                })}
              </p>
            </li>
          ))}
        </ul>
        <p className="text-muted-foreground border-t pt-2 text-xs">
          {t("agentSessions.costPricing")}
        </p>
      </PopoverContent>
    </Popover>
  );
}
