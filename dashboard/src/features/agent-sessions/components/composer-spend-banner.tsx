import { Link } from "@tanstack/react-router";
import { AlertTriangle, OctagonX } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import type { ComposerSpendGate } from "@/features/agent-environment/lib/spend";
import { money } from "@/features/usage/lib/charges";

const LINK_CLASS = "font-medium underline underline-offset-4";

/**
 * The composer's spend banner: a warning at 80% of the monthly cap; at the
 * cap a refusal worded by its cause (the workspace's own cap, the
 * new-workspace limit, or a payment hold) with an "Increase cap" link for
 * admins only while the workspace's own cap binds below its maximum; and while a failed invoice is unpaid, a refusal that links to
 * Billing. The mint would refuse those sessions' model requests anyway.
 */
export function ComposerSpendBanner({ gate }: { gate: ComposerSpendGate }) {
  const { t, i18n } = useTranslations();
  const { canManage } = useCapabilities();
  const usd = (value: number) => money(value, i18n.language);

  switch (gate.kind) {
    case "none":
      return null;
    case "balanceDue":
      return (
        <Alert variant="destructive" data-testid="composer-spend-banner">
          <OctagonX />
          <AlertTitle>{t("agentSessions.spendBalanceDueTitle")}</AlertTitle>
          <AlertDescription>
            <p>{t("agentSessions.spendBalanceDueBody")}</p>
            <Link to="/billing" className={LINK_CLASS}>
              {t("agentSessions.spendGoToBilling")}
            </Link>
          </AlertDescription>
        </Alert>
      );
    case "warning": {
      const { position } = gate;
      return (
        <Alert data-testid="composer-spend-banner">
          <AlertTriangle />
          <AlertTitle>
            {t("agentSessions.spendWarningTitle", {
              percent: Math.floor(position.fraction * 100),
            })}
          </AlertTitle>
          <AlertDescription>
            <p>
              {t(
                position.chargePoint
                  ? "agentSessions.spendWarningBodyCharge"
                  : "agentSessions.spendWarningBody",
                {
                  used: usd(position.monthToDateUsd),
                  cap: usd(position.capUsd),
                },
              )}
            </p>
          </AlertDescription>
        </Alert>
      );
    }
    case "reached": {
      const { position, cause, canRaiseCap } = gate;
      // The workspace's own cap at its maximum has nothing left to raise.
      const body = cause === "cap" && !canRaiseCap ? "capMax" : cause;
      const params = {
        used: usd(position.monthToDateUsd),
        cap: usd(position.capUsd),
      };
      return (
        <Alert variant="destructive" data-testid="composer-spend-banner">
          <OctagonX />
          <AlertTitle>
            {t(`agentSessions.spendReachedTitle.${cause}`)}
          </AlertTitle>
          <AlertDescription>
            <p>{t(`agentSessions.spendReachedBody.${body}`, params)}</p>
            {canRaiseCap && canManage ? (
              <Link
                to="/workspace/settings"
                hash="agents"
                className={LINK_CLASS}
              >
                {t("agentSessions.spendIncreaseCap")}
              </Link>
            ) : cause === "locked" ? (
              <Link to="/billing" className={LINK_CLASS}>
                {t("agentSessions.spendGoToBilling")}
              </Link>
            ) : null}
          </AlertDescription>
        </Alert>
      );
    }
  }
}
