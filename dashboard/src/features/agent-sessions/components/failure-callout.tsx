import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import { AlertCircle, ArrowUpCircle, RotateCcw, Wrench } from "lucide-react";
import { toast } from "sonner";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import { agentSessionErrorMessage } from "@/features/agent-sessions/lib/errors";
import {
  stoppedAtSessionSpendLimit,
  formatPreciseUsd,
} from "@/features/agent-sessions/lib/spend-limit";
import {
  agentSessionFailureReason,
  isSandboxCapacityFailure,
  isRepoSetupFailure,
} from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { REPO_ENVIRONMENTS_ANCHOR } from "@/features/agent-environment/lib/repo-environments";

const SPEND_FAILURE_KEYS: Record<string, string> = {
  AGENT_MODEL_SPEND_CAP_REACHED: "agentSessions.failureSpendCapReached",
  BILLING_BALANCE_DUE: "agentSessions.failureBalanceDue",
};

export interface FailureCalloutProps {
  session: AgentSessionView;
  /** Re-read the session once a retry redispatch converges (phase/turns change). */
  onRetried?: () => void;
}

/**
 * The inline failure callout for a `failed` session (w2/m64). It surfaces the
 * recorded reason — `failureReason`, else the lifecycle `status` where a
 * provisioning failure was stamped before w5/m80, else a generic fallback so the
 * callout is never empty or filled with noise (see `agentSessionFailureReason`)
 * — and offers a one-click **Retry** that re-runs the session's original task
 * through the steer (redispatch) mutation. A sandbox-capacity failure
 * (`isSandboxCapacityFailure`) leads with an **Upgrade plan** action instead,
 * and a failed repository setup step (ADR109 D5) keeps its recorded reason —
 * which names the step and its exit code — and leads to the repository's
 * environment in settings, where the step is fixed.
 * Retry rides the same fast, accept-then-provision path, so the button releases
 * as soon as the new turn is accepted; the header refetch then surfaces the
 * redispatching phase.
 */
export function FailureCallout({ session, onRetried }: FailureCalloutProps) {
  const { t, i18n } = useTranslations();
  const navigate = useNavigate();
  const { steer } = useAgentSessionMutations();
  const [retrying, setRetrying] = useState(false);

  if (session.phase !== "failed") return null;

  // A plan-limit refusal is fixed by upgrading, not by retrying into the same
  // wall — so lead with an Upgrade CTA (Retry stays for the "freed a sandbox"
  // case). Any other failure keeps the recorded reason + Retry.
  const capacity = isSandboxCapacityFailure(session);
  const setupFailed = isRepoSetupFailure(session);
  // The model mint's typed refusals (ADR091 D5, D10.3) read in the user's
  // language rather than as the server's English sentence.
  const spendKey = SPEND_FAILURE_KEYS[session.failureReasonCode ?? ""];
  // A session that hit its own maxSpendUSD ceiling names it, not the monthly cap.
  const reason = capacity
    ? t("agentSessions.capacityFailureBody")
    : stoppedAtSessionSpendLimit(session)
      ? t("agentSessions.failureSessionSpendLimitReached", {
          limit: formatPreciseUsd(session.maxSpendUSD ?? "", i18n.language),
        })
      : spendKey
        ? t(spendKey)
        : (agentSessionFailureReason(session) ??
          t("agentSessions.failureReasonFallback"));

  async function handleRetry() {
    setRetrying(true);
    try {
      const sent = await steer(session.id, session.agentConfig.task);
      // Messages already waiting run first: the retry then joins them (D12).
      toast.success(
        t(
          sent.queuedMessage
            ? "agentSessions.steerQueued"
            : "agentSessions.steerSuccess",
        ),
      );
      onRetried?.();
    } catch (err) {
      // Closing the ADR075 D7 payment dialog is a user choice, not a failure.
      if (!isPaymentOnboardingCancelled(err)) {
        toast.error(agentSessionErrorMessage(err, t));
      }
    } finally {
      setRetrying(false);
    }
  }

  return (
    <Alert variant="destructive">
      <AlertCircle />
      <AlertTitle>
        {capacity
          ? t("agentSessions.capacityFailureTitle")
          : setupFailed
            ? t("agentSessions.failureSetupTitle")
            : t("agentSessions.failureTitle")}
      </AlertTitle>
      <AlertDescription className="flex flex-col items-start gap-2">
        <span>{reason}</span>
        {setupFailed ? (
          <span>{t("agentSessions.failureSetupHint")}</span>
        ) : null}
        <div className="flex flex-wrap gap-2">
          {setupFailed ? (
            <Button
              type="button"
              size="sm"
              onClick={() =>
                void navigate({
                  to: "/workspace/settings",
                  hash: REPO_ENVIRONMENTS_ANCHOR,
                })
              }
            >
              <Wrench className="size-4" />
              {t("agentSessions.failureSetupAction")}
            </Button>
          ) : null}
          {capacity ? (
            <Button
              type="button"
              size="sm"
              onClick={() =>
                void navigate({
                  to: "/workspace/settings",
                  search: { plan: "change" },
                })
              }
            >
              <ArrowUpCircle className="size-4" />
              {t("agentSessions.upgradePlan")}
            </Button>
          ) : null}
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={retrying}
            onClick={() => void handleRetry()}
          >
            <RotateCcw className="size-4" />
            {retrying
              ? t("agentSessions.failureRetrying")
              : t("agentSessions.failureRetry")}
          </Button>
        </div>
      </AlertDescription>
    </Alert>
  );
}
