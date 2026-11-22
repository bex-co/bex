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

/**
 * A model id's label in the model list (w10/m527), shared by the ready card
 * and this skeleton: monospace, breaking anywhere on a narrow screen, one
 * line as tall as the switch beside it.
 */
export const MODEL_LABEL_CLASS = "font-mono leading-5 break-all";

// The skeleton's model list: a representative row count (the platform
// catalog's size at w10/m527) and a hidden id as long as its longest model.
const MODEL_ROWS = 4;
const MODEL_ID_PLACEHOLDER = "provider/model-name-version-00000";

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

/** A list of switch rows (available agents, models). */
export function SwitchList({ children }: { children: ReactNode }) {
  return <ul className="max-w-xl space-y-3">{children}</ul>;
}

/**
 * One switch row's frame, shared by the ready sections and this skeleton:
 * the label with an optional note under it, and the control (the switch, or
 * its placeholder) beside them.
 */
export function SwitchListRow({
  label,
  note,
  noteId,
  control,
}: {
  label: ReactNode;
  note?: ReactNode;
  noteId?: string;
  control: ReactNode;
}) {
  return (
    <li className="flex items-start justify-between gap-4">
      <div className="min-w-0 space-y-0.5">
        {label}
        {note ? (
          <p id={noteId} className="text-muted-foreground text-xs">
            {note}
          </p>
        ) : null}
      </div>
      {control}
    </li>
  );
}

/**
 * A setting switch's row in the GitHub CLI section (the workspace switch,
 * w10/m524, and its mandatory flag, w10/m561), shared by the ready section
 * and this skeleton: the label and its help beside the control.
 */
export function SettingSwitchRow({
  label,
  help,
  helpId,
  control,
}: {
  label: ReactNode;
  help: ReactNode;
  helpId?: string;
  control: ReactNode;
}) {
  return (
    <div className="flex max-w-xl items-start justify-between gap-4">
      <div className="min-w-0 space-y-1">
        {label}
        <p id={helpId} className="text-muted-foreground text-sm">
          {help}
        </p>
      </div>
      {control}
    </div>
  );
}

/**
 * A setting picked from a list in the GitHub CLI section (the automations
 * default, w10/m561), shared by the ready section and this skeleton: the
 * label over the control, its help, then its error when a save is refused.
 */
export function SettingSelectField({
  label,
  control,
  help,
  helpId,
  error,
}: {
  label: ReactNode;
  control: ReactNode;
  help: ReactNode;
  helpId?: string;
  error?: ReactNode;
}) {
  return (
    <div className="max-w-xl space-y-2">
      {label}
      {control}
      <p id={helpId} className="text-muted-foreground text-xs">
        {help}
      </p>
      {error}
    </div>
  );
}

/** A setting select's box (h-9, as wide as the ready trigger) while it loads. */
export const SETTING_SELECT_CLASS = "w-full max-w-sm";

/** A switch's box (h-5 w-9) while its value loads. */
function SwitchPlaceholder() {
  return <Skeleton className="h-5 w-9 shrink-0 rounded-full" />;
}

/**
 * The skeleton's switch rows: each label and note as given, a switch
 * placeholder beside them.
 */
function SkeletonSwitchList({
  rows,
}: {
  rows: { key: string; label: ReactNode; note?: ReactNode }[];
}) {
  return (
    <SwitchList>
      {rows.map(({ key, label, note }) => (
        <SwitchListRow
          key={key}
          label={label}
          note={note}
          control={<SwitchPlaceholder />}
        />
      ))}
    </SwitchList>
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
function StaticLabel({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <Label asChild className={className}>
      <span>{children}</span>
    </Label>
  );
}

// Space-holding amounts for hidden placeholder lines: wide enough for any
// realistic cap or charge point, never shown.
const WIDE_USD = "$1,000.00";
const WIDER_USD = "$10,000.00";
const longest = (...texts: string[]) =>
  texts.reduce((a, b) => (b.length > a.length ? b : a));

/**
 * The Settings → Agents card while its environment loads, at the ready card's
 * geometry at every width (dashboard/AGENTS.md skeleton rule). Static copy —
 * titles, descriptions, tier options, labels, help, the pricing statement and
 * button labels — renders as real text through the same pieces the ready card
 * uses, so it wraps identically. Only data-dependent values are skeletons, and
 * a value that can wrap holds its longest variant's line count. The Custom
 * hostname list and Remove key are data-dependent and omitted, and the model
 * list holds a representative count of rows; every action is disabled until
 * the data arrives. `readOnly` is a caller known not to manage the workspace,
 * whose card opens with the Admin-only note (w7/094).
 */
export function AgentEnvironmentCardSkeleton({
  readOnly = false,
}: {
  readOnly?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <Card data-testid="agent-environment-skeleton">
      <CardHeader>
        <CardTitle>{t("agentEnvironment.title")}</CardTitle>
        <CardDescription>{t("agentEnvironment.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {readOnly ? (
          <p className="text-sm">
            <SkeletonText>{t("agentEnvironment.readOnlyNote")}</SkeletonText>
          </p>
        ) : null}
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
        {/* The sessions' GitHub CLI switch (w10/m524), its mandatory flag
            and the automations default (w10/m561). */}
        <div
          className="space-y-3 border-t pt-6"
          data-testid="github-cli-section-skeleton"
        >
          <SettingSwitchRow
            label={
              <StaticLabel>{t("agentEnvironment.githubCliLabel")}</StaticLabel>
            }
            help={t("agentEnvironment.githubCliHelp")}
            control={<SwitchPlaceholder />}
          />
          <SettingSwitchRow
            label={
              <StaticLabel>
                {t("agentEnvironment.githubCliMandatoryLabel")}
              </StaticLabel>
            }
            help={t("agentEnvironment.githubCliMandatoryHelp")}
            control={<SwitchPlaceholder />}
          />
          <SettingSelectField
            label={
              <StaticLabel>
                {t("agentEnvironment.githubCliAutomationsLabel")}
              </StaticLabel>
            }
            control={<Skeleton className={cn("h-9", SETTING_SELECT_CLASS)} />}
            help={t("agentEnvironment.githubCliAutomationsHelp")}
          />
        </div>
        <div className="space-y-2 border-t pt-6">
          <StaticLabel>{t("agentEnvironment.modelLabel")}</StaticLabel>
          <Skeleton className="h-9 w-full max-w-sm" />
          {/* The help line depends on whether the platform route offers its
              model catalog (w10/m516), so hold the longer one. */}
          <p className="text-muted-foreground text-xs">
            <SkeletonText>
              {longest(
                t("agentEnvironment.modelHelp"),
                t("agentEnvironment.modelCatalogHelp"),
              )}
            </SkeletonText>
          </p>
        </div>
        {/* The models members can choose (w10/m527). The ready card draws
            them on the platform catalog only, the case the default-model
            help above holds: representative rows, the first the catalog's
            locked default with its static note. */}
        <div
          className="space-y-3 border-t pt-6"
          data-testid="enabled-models-section-skeleton"
        >
          <AgentSectionHeading
            title={t("agentEnvironment.modelsTitle")}
            description={t("agentEnvironment.modelsDescription")}
          />
          <SkeletonSwitchList
            rows={Array.from({ length: MODEL_ROWS }, (_, i) => ({
              key: String(i),
              label: (
                <StaticLabel className={MODEL_LABEL_CLASS}>
                  <span>
                    <SkeletonText>{MODEL_ID_PLACEHOLDER}</SkeletonText>
                  </span>
                </StaticLabel>
              ),
              note: i === 0 ? t("agentEnvironment.modelsDefaultLocked") : null,
            }))}
          />
        </div>
        {/* The default sandbox template (w10/m519). */}
        <div
          className="space-y-2 border-t pt-6"
          data-testid="default-template-section-skeleton"
        >
          <StaticLabel>{t("agentEnvironment.templateLabel")}</StaticLabel>
          <Skeleton className="h-9 w-full max-w-sm" />
          <p className="text-muted-foreground text-xs">
            {t("agentEnvironment.templateHelp")}
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
            {/* The summary holds its longer, beyond-credits variant. */}
            <p className="text-sm font-medium">
              <SkeletonText>
                {longest(
                  t("agentEnvironment.spendSummary", {
                    used: WIDE_USD,
                    cap: WIDE_USD,
                  }),
                  t("agentEnvironment.spendSummaryCeiling", {
                    used: WIDE_USD,
                    cap: WIDE_USD,
                    total: WIDE_USD,
                  }),
                )}
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
                  t("agentEnvironment.spendTierNextChargeCredit", {
                    tier: "T0",
                    amount: WIDE_USD,
                    credit: WIDE_USD,
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
          <SkeletonSwitchList
            rows={AGENT_OPTIONS.map((agent) => ({
              key: agent,
              label: (
                <StaticLabel>{t(`agentSessions.agent.${agent}`)}</StaticLabel>
              ),
              note: DATA_USE_AGENTS.has(agent) ? (
                <SkeletonText>
                  {t("agentEnvironment.agentsDataUseNote")}
                </SkeletonText>
              ) : null,
            }))}
          />
        </div>
      </CardContent>
      <CardFooter className="justify-end border-t">
        <Button disabled>{t("agentEnvironment.save")}</Button>
      </CardFooter>
    </Card>
  );
}
