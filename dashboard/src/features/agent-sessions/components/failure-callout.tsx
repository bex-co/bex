import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import { AlertCircle, ArrowUpCircle, RotateCcw } from "lucide-react";
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
  agentSessionFailureReason,
  isSandboxCapacityFailure,
} from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";

export interface FailureCalloutProps {
  session: AgentSessionView;
  /** Re-read the session once a retry redispatch converges (phase/turns change). */
  onRetried?: () => void;
}

/**
 * The inline failure callout for a `failed` session (w2/m64). It surfaces the
 * recorded reason — `failureReason` when the Completer named one, else the
 * lifecycle `status` a background-provisioning failure stamps ("sandbox create
 * failed"), else a generic fallback so the callout is never empty or filled with
 * noise (see `agentSessionFailureReason`) — and offers a
 * one-click **Retry** that re-runs the session's original task through the steer
 * (redispatch) mutation. Retry rides the same fast, accept-then-provision path,
 * so the button releases as soon as the new turn is accepted; the header refetch
 * then surfaces the redispatching phase.
 */
export function FailureCallout({ session, onRetried }: FailureCalloutProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { steer } = useAgentSessionMutations();
  const [retrying, setRetrying] = useState(false);

  if (session.phase !== "failed") return null;

  // A plan-limit refusal is fixed by upgrading, not by retrying into the same
  // wall — so lead with an Upgrade CTA (Retry stays for the "freed a sandbox"
  // case). Any other failure keeps the recorded reason + Retry.
  const capacity = isSandboxCapacityFailure(session);
  const reason = capacity
    ? t("agentSessions.capacityFailureBody")
    : (agentSessionFailureReason(session) ??
      t("agentSessions.failureReasonFallback"));

  async function handleRetry() {
    setRetrying(true);
    try {
      await steer(session.id, session.agentConfig.task);
      toast.success(t("agentSessions.steerSuccess"));
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
          : t("agentSessions.failureTitle")}
      </AlertTitle>
      <AlertDescription className="flex flex-col items-start gap-2">
        <span>{reason}</span>
        <div className="flex flex-wrap gap-2">
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
