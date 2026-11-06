import { useId, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { UsageMeter } from "@/common/components/usage-meter";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  isForbiddenError,
} from "@/common/lib/graphql-error";
import { money } from "@/features/usage/lib/charges";
import type { AgentEnvironmentInput } from "@/graphql/definitions";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";
import {
  NEW_WORKSPACE_TIER,
  parseSpendCap,
  spendPosition,
} from "@/features/agent-environment/lib/spend";
import { AgentSectionHeading } from "@/features/agent-environment/components/agent-environment-card-skeleton";

interface SpendCapSectionProps {
  saved: AgentEnvironmentView;
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>;
  canManage: boolean;
}

function capDraft(env: AgentEnvironmentView): string {
  const cap =
    env.agentModelSpendCapOverrideUsd ?? env.agentModelPlanSpendCapUsd;
  return cap == null ? "" : String(cap);
}

/**
 * Settings → Agents' agent-model spend (ADR091 D5, D6, D10.1): month-to-date
 * spend against the effective cap, the pricing statement, the billing trust
 * tier with its next charge point, and the admin-editable monthly cap (up to
 * 10x the plan default; Reset returns to the plan default). Non-admins see the
 * same controls disabled, so the loaded section keeps its skeleton's geometry.
 */
export function SpendCapSection({
  saved,
  update,
  canManage,
}: SpendCapSectionProps) {
  const { t, i18n } = useTranslations();
  const inputId = `${useId()}-spend-cap`;
  const usd = (value: number) => money(value, i18n.language);
  const [draft, setDraft] = useState(() => capDraft(saved));
  const [base, setBase] = useState(saved);
  if (base !== saved) {
    setBase(saved);
    setDraft(capDraft(saved));
  }
  const [busy, setBusy] = useState<"save" | "reset" | null>(null);
  const [errorKey, setErrorKey] = useState<string | null>(null);

  const position = spendPosition(saved);
  const maxUsd = saved.agentModelMaxSpendCapUsd;
  const planUsd = saved.agentModelPlanSpendCapUsd;
  const value = parseSpendCap(draft, maxUsd);
  const invalid = draft.trim() !== "" && value == null;
  const unchanged = draft === capDraft(saved);
  const shownError = invalid ? "agentEnvironment.spendCapInvalid" : errorKey;
  const helpId = `${inputId}-help`;
  const errorId = `${inputId}-error`;

  async function write(input: AgentEnvironmentInput, doneKey: string) {
    setErrorKey(null);
    setBusy(input.resetAgentModelMonthlySpendCap ? "reset" : "save");
    try {
      await update(input);
      toast.success(t(doneKey));
    } catch (err) {
      setErrorKey(
        hasGraphQLErrorCode(err, "AGENT_ENVIRONMENT_SPEND_CAP_INVALID")
          ? "agentEnvironment.spendCapInvalid"
          : isForbiddenError(err)
            ? "agentEnvironment.forbidden"
            : "agentEnvironment.spendCapSaveFailed",
      );
    } finally {
      setBusy(null);
    }
  }

  const tier = saved.trustTier;
  const next = saved.nextChargeThresholdUsd;
  const tierLine = tier
    ? next != null
      ? t("agentEnvironment.spendTierNextCharge", { tier, amount: usd(next) })
      : t("agentEnvironment.spendTierMonthEnd", { tier })
    : null;
  const tierNote = saved.trustTierLocked
    ? t("agentEnvironment.spendTierLocked")
    : tier === NEW_WORKSPACE_TIER
      ? t("agentEnvironment.spendTierNew")
      : null;

  return (
    <div className="space-y-3 border-t pt-6" data-testid="spend-cap-section">
      <AgentSectionHeading
        title={t("agentEnvironment.spendTitle")}
        description={t("agentEnvironment.spendPricing")}
      />
      <div className="space-y-2">
        <p className="text-sm font-medium" data-testid="spend-summary">
          {position
            ? t("agentEnvironment.spendSummary", {
                used: usd(position.monthToDateUsd),
                cap: usd(position.capUsd),
              })
            : t("agentEnvironment.spendUnavailable")}
        </p>
        <UsageMeter
          label={t("agentEnvironment.spendTitle")}
          fraction={position?.fraction ?? null}
          level={position?.level ?? "ok"}
          valueText={
            position
              ? t("agentEnvironment.spendMeterValue", {
                  used: usd(position.monthToDateUsd),
                  cap: usd(position.capUsd),
                })
              : t("agentEnvironment.spendUnavailable")
          }
        />
        <p className="text-muted-foreground text-xs" data-testid="spend-tier">
          {tierLine ?? " "}
        </p>
        {/* Its own line, always reserved, so the T0 / payment-hold note never
            reflows the tier line or shifts the cap field below it. */}
        <p
          className="text-muted-foreground text-xs"
          data-testid="spend-tier-note"
        >
          {tierNote ?? " "}
        </p>
      </div>
      <div className="space-y-2">
        <Label htmlFor={inputId}>{t("agentEnvironment.spendCapLabel")}</Label>
        <div className="flex max-w-xl flex-wrap gap-2">
          <Input
            id={inputId}
            type="number"
            inputMode="decimal"
            min={0.01}
            step={0.01}
            max={maxUsd ?? undefined}
            value={draft}
            onChange={(e) => {
              setDraft(e.target.value);
              setErrorKey(null);
            }}
            disabled={!canManage}
            aria-invalid={shownError != null}
            aria-describedby={shownError ? `${helpId} ${errorId}` : helpId}
            className="max-w-40 tabular-nums"
          />
          <Button
            type="button"
            variant="outline"
            className="min-w-24"
            disabled={!canManage || value == null || unchanged || busy != null}
            onClick={() =>
              value != null &&
              void write(
                { agentModelMonthlySpendCapUsd: value },
                "agentEnvironment.spendCapSaved",
              )
            }
          >
            {busy === "save" ? <Loader2 className="animate-spin" /> : null}
            {t("agentEnvironment.spendCapSave")}
          </Button>
          <Button
            type="button"
            variant="ghost"
            className="min-w-48"
            disabled={
              !canManage ||
              saved.agentModelSpendCapOverrideUsd == null ||
              busy != null
            }
            onClick={() =>
              void write(
                { resetAgentModelMonthlySpendCap: true },
                "agentEnvironment.spendCapResetDone",
              )
            }
          >
            {busy === "reset" ? <Loader2 className="animate-spin" /> : null}
            {t("agentEnvironment.spendCapReset")}
          </Button>
        </div>
        <p id={helpId} className="text-muted-foreground text-xs">
          {planUsd != null && maxUsd != null
            ? t("agentEnvironment.spendCapHelp", {
                plan: usd(planUsd),
                max: usd(maxUsd),
              })
            : " "}
        </p>
        {shownError ? (
          <p id={errorId} className="text-destructive text-sm" role="alert">
            {t(shownError, { max: maxUsd != null ? usd(maxUsd) : "" })}
          </p>
        ) : null}
      </div>
    </div>
  );
}
