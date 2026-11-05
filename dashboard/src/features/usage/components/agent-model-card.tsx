// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { useId, useState } from "react";
import { ChevronRight } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import { money, usd } from "@/features/usage/lib/charges";
import type { AgentModelUsage } from "../hooks/use-usage";
import { AgentModelCardSkeleton } from "./agent-model-card-skeleton";

const TOKEN_KINDS = [
  ["inputTokens", "usage.chargeAgentModelInputTokens"],
  ["outputTokens", "usage.chargeAgentModelOutputTokens"],
  ["cacheReadTokens", "usage.chargeAgentModelCacheReadTokens"],
  ["cacheWriteTokens", "usage.chargeAgentModelCacheWriteTokens"],
] as const;

/**
 * The "Agent model" line of the billing page (ADR091 D6): the period's
 * bex-provided model spend, billed at the model provider's cost + 30%, with
 * an expandable breakdown by model and token kind. Always rendered (a
 * workspace with no agent usage reads $0.00), so the page's geometry is
 * stable; the breakdown starts collapsed.
 */
export function AgentModelCard({
  agentModel,
  loading,
  current,
}: {
  agentModel: AgentModelUsage | null;
  loading: boolean;
  /** The period is the current month (month to date), not a closed one. */
  current: boolean;
}) {
  const { t, i18n } = useTranslations();
  const [open, setOpen] = useState(false);
  const breakdownId = useId();
  if (loading && !agentModel) return <AgentModelCardSkeleton />;
  const models = agentModel?.models ?? [];
  const number = new Intl.NumberFormat(i18n.language);
  const amount = money(usd(agentModel?.monthToDateUsd ?? "0"), i18n.language);

  return (
    <Card data-testid="agent-model-card">
      <CardHeader>
        <CardTitle>{t("usage.agentModelTitle")}</CardTitle>
        <CardDescription>{t("usage.agentModelPricing")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-baseline justify-between gap-4">
          <span className="text-muted-foreground text-sm">
            {current
              ? t("usage.agentModelMonthToDate")
              : t("usage.agentModelPeriodTotal")}
          </span>
          <span
            className="font-mono text-lg font-semibold tabular-nums"
            data-testid="agent-model-total"
          >
            {amount}
          </span>
        </div>
        {models.length > 0 ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="-ml-2 h-8 gap-1"
            aria-expanded={open}
            aria-controls={breakdownId}
            onClick={() => setOpen((v) => !v)}
          >
            <ChevronRight
              className={cn("size-4 transition-transform", open && "rotate-90")}
            />
            {t("usage.agentModelBreakdown", { count: models.length })}
          </Button>
        ) : (
          <p className="text-muted-foreground flex h-8 items-center text-sm">
            {t("usage.agentModelEmpty")}
          </p>
        )}
        {models.length > 0 ? (
          <div
            id={breakdownId}
            hidden={!open}
            className="divide-y rounded-md border"
          >
            {models.map((m) => (
              <div key={m.model} className="space-y-1 px-3 py-2">
                <div className="flex items-baseline justify-between gap-4">
                  <span className="truncate font-mono text-sm" title={m.model}>
                    {m.model}
                  </span>
                  <span className="font-mono text-sm tabular-nums">
                    {money(usd(m.costUsd), i18n.language)}
                  </span>
                </div>
                <dl className="text-muted-foreground grid grid-cols-2 gap-x-4 gap-y-0.5 text-xs sm:grid-cols-4">
                  {TOKEN_KINDS.map(([field, labelKey]) => (
                    <div key={field} className="flex justify-between gap-2">
                      <dt>{t(labelKey)}</dt>
                      <dd className="font-mono tabular-nums">
                        {number.format(m[field])}
                      </dd>
                    </div>
                  ))}
                </dl>
              </div>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
