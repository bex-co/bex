import type { ComponentProps, ReactNode } from "react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { TIERS } from "@/features/agent-environment/lib/environment";
import { PricingWithTerms } from "@/features/legal/components/agent-model-terms-link";
import {
  AGENT_OPTIONS,
  DATA_USE_AGENTS,
} from "@/features/agent-sessions/lib/composer";

/** One network-tier option's frame, shared by the ready card and this skeleton. */
export const TIER_OPTION_CLASS =
  "flex min-h-20 w-full flex-col gap-1 rounded-lg border p-4";

/** A section's heading and description (network, model provider, spend). */
export function AgentSectionHeading({
  title,
  description,
}: {
  title: string;
  description: ReactNode;
}) {
  return (
    <div className="space-y-1">
      <h3 className="text-sm font-medium">{title}</h3>
      <p className="text-muted-foreground text-sm">{description}</p>
    </div>
  );
}

/** A network-tier option's title and description, inside its frame. */
export function TierOptionText({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <>
      <span className="flex items-center gap-2 text-sm font-medium">
        {title}
      </span>
      <span className="text-muted-foreground text-sm">{description}</span>
    </>
  );
}

/** "Model source: <value>", the value being the source or its placeholder. */
export function ModelSourceLine({ children, ...props }: ComponentProps<"p">) {
  const { t } = useTranslations();
  return (
    <p className="text-sm" {...props}>
      <span className="text-muted-foreground">
        {t("agentEnvironment.modelSourceLabel")}{" "}
      </span>
      <span className="font-medium">{children}</span>
    </p>
  );
}

/**
 * An inline placeholder for a data-dependent line: the text is transparent
 * and hidden from assistive tech, so it holds exactly the lines the value can
 * wrap to at the current width while each line reads as a skeleton bar.
 */
function SkeletonText({ children }: { children: ReactNode }) {
  return (
    <span
      aria-hidden="true"
      data-slot="skeleton-text"
      className="bg-accent animate-pulse rounded-md box-decoration-clone text-transparent select-none motion-reduce:animate-none group-data-[skeleton-frame=true]:!animate-none"
    >
      {children}
    </span>
  );
}

/** A static label in a label's typography, with no control to point at. */
function StaticLabel({ children }: { children: ReactNode }) {
  return (
    <Label asChild>
      <span>{children}</span>
    </Label>
  );
}

// Space-holding amounts for hidden placeholder lines: wide enough for any
// realistic cap or charge point, never shown.
const WIDE_USD = "$1,000.00";
const WIDER_USD = "$10,000.00";
// A realistic platform-pinned model id, for the pinned help text's length.
const PINNED_MODEL = "deepseek/deepseek-v4.1-flash-1017";

const longest = (...texts: string[]) =>
  texts.reduce((a, b) => (b.length > a.length ? b : a));

/**
 * The Settings → Agents card while its environment loads, at the ready card's
 * geometry at every width (dashboard/AGENTS.md skeleton rule). Static copy —
 * titles, descriptions, tier options, labels, help, the pricing statement and
 * button labels — renders as real text through the same pieces the ready card
 * uses, so it wraps identically. Only data-dependent values are skeletons, and
 * a value that can wrap holds its longest variant's line count. The Custom
 * hostname list, the read-only note and Remove key are data-dependent and
 * omitted; every action is disabled until the data arrives.
 */
export function AgentEnvironmentCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="agent-environment-skeleton">
      <CardHeader>
        <CardTitle>{t("agentEnvironment.title")}</CardTitle>
        <CardDescription>{t("agentEnvironment.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-3">
          <AgentSectionHeading
            title={t("agentEnvironment.networkTitle")}
            description={t("agentEnvironment.networkDescription")}
          />
          <div className="grid grid-cols-1 gap-3">
            {TIERS.map(({ tier, titleKey, descriptionKey }) => (
              <div
                key={tier}
                className={cn(TIER_OPTION_CLASS, "border-border")}
              >
                <TierOptionText
                  title={t(titleKey)}
                  description={t(descriptionKey)}
                />
              </div>
            ))}
          </div>
          <p className="text-sm">
            <SkeletonText>
              {t("agentEnvironment.registryHostsSummary", { count: 99 })}
            </SkeletonText>
          </p>
        </div>
        <div className="space-y-2 border-t pt-6">
          <StaticLabel>{t("agentEnvironment.modelLabel")}</StaticLabel>
          <Skeleton className="h-9 w-full max-w-sm" />
          {/* The help line depends on whether the platform route pins the
              model, and the pinned variant names it, so hold the longer one. */}
          <p className="text-muted-foreground text-xs">
            <SkeletonText>
              {longest(
                t("agentEnvironment.modelHelp"),
                t("agentEnvironment.modelPinnedHelp", { model: PINNED_MODEL }),
              )}
            </SkeletonText>
          </p>
        </div>
        <div
          className="space-y-3 border-t pt-6"
          data-testid="model-key-section-skeleton"
        >
          <AgentSectionHeading
            title={t("agentEnvironment.modelKeyTitle")}
            description={t("agentEnvironment.modelKeyDescription")}
          />
          <ModelSourceLine>
            <SkeletonText>
              {longest(
                t("agentEnvironment.modelSourcePlatform"),
                t("agentEnvironment.modelSourceByo"),
                t("agentEnvironment.modelSourceNone"),
              )}
            </SkeletonText>
          </ModelSourceLine>
          <div className="space-y-2">
            <StaticLabel>
              <SkeletonText>
                {longest(
                  t("agentEnvironment.modelKeyLabel"),
                  t("agentEnvironment.modelKeyReplaceLabel"),
                )}
              </SkeletonText>
            </StaticLabel>
            <div className="flex max-w-xl flex-wrap gap-2">
              <Skeleton className="h-9 max-w-sm flex-1" />
              <Button type="button" variant="outline" disabled>
                {t("agentEnvironment.modelKeySave")}
              </Button>
            </div>
            <p className="text-muted-foreground text-xs">
              {t("agentEnvironment.modelKeyHelp")}
            </p>
          </div>
        </div>
        <div
          className="space-y-3 border-t pt-6"
          data-testid="spend-cap-section-skeleton"
        >
          <AgentSectionHeading
            title={t("agentEnvironment.spendTitle")}
            description={
              <PricingWithTerms
                pricing={t("agentEnvironment.spendPricing")}
                placeholder
              />
            }
          />
          <div className="space-y-2">
            <p className="text-sm font-medium">
              <SkeletonText>
                {t("agentEnvironment.spendSummary", {
                  used: WIDE_USD,
                  cap: WIDE_USD,
                })}
              </SkeletonText>
            </p>
            <Skeleton className="h-2 w-full rounded-full" />
            {/* Tier line, then the T0 / payment-hold note on its own line,
                each holding its longest variant. */}
            <p className="text-muted-foreground text-xs">
              <SkeletonText>
                {longest(
                  t("agentEnvironment.spendTierNextCharge", {
                    tier: "T0",
                    amount: WIDE_USD,
                  }),
                  t("agentEnvironment.spendTierMonthEnd", { tier: "T0" }),
                )}
              </SkeletonText>
            </p>
            <p className="text-muted-foreground text-xs">
              <SkeletonText>
                {longest(
                  t("agentEnvironment.spendTierNew", { amount: WIDE_USD }),
                  t("agentEnvironment.spendTierLocked"),
                )}
              </SkeletonText>
            </p>
          </div>
          <div className="space-y-2">
            <StaticLabel>{t("agentEnvironment.spendCapLabel")}</StaticLabel>
            <div className="flex max-w-xl flex-wrap gap-2">
              <Skeleton className="h-9 w-40" />
              <Button
                type="button"
                variant="outline"
                className="min-w-24"
                disabled
              >
                {t("agentEnvironment.spendCapSave")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="min-w-48"
                disabled
              >
                {t("agentEnvironment.spendCapReset")}
              </Button>
            </div>
            <p className="text-muted-foreground text-xs">
              <SkeletonText>
                {t("agentEnvironment.spendCapHelp", {
                  plan: WIDE_USD,
                  max: WIDER_USD,
                })}
              </SkeletonText>
            </p>
          </div>
        </div>
        <div
          className="space-y-3 border-t pt-6"
          data-testid="available-agents-section-skeleton"
        >
          <AgentSectionHeading
            title={t("agentEnvironment.agentsTitle")}
            description={t("agentEnvironment.agentsDescription")}
          />
          <ul className="max-w-xl space-y-3">
            {AGENT_OPTIONS.map((agent) => (
              <li
                key={agent}
                className="flex items-start justify-between gap-4"
              >
                <div className="min-w-0 space-y-0.5">
                  <StaticLabel>{t(`agentSessions.agent.${agent}`)}</StaticLabel>
                  {DATA_USE_AGENTS.has(agent) ? (
                    <p className="text-muted-foreground text-xs">
                      <SkeletonText>
                        {t("agentEnvironment.agentsDataUseNote")}
                      </SkeletonText>
                    </p>
                  ) : null}
                </div>
                <Skeleton className="h-5 w-9 shrink-0 rounded-full" />
              </li>
            ))}
          </ul>
        </div>
      </CardContent>
      <CardFooter className="justify-end border-t">
        <Button disabled>{t("agentEnvironment.save")}</Button>
      </CardFooter>
    </Card>
  );
}
