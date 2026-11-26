import { useCallback, useEffect, useId, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { useNavigate } from "@tanstack/react-router";
import { Info, Loader2, TriangleAlert } from "lucide-react";
import { toast } from "sonner";
import {
  RollbackComparisonDocument,
  RollbackServiceDocument,
} from "@/graphql/definitions";
import { Button } from "@/common/components/ui/button";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { DEPLOY_REFETCH_QUERIES } from "@/common/lib/fetch-policy";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import { useServiceBase } from "@/features/services/lib/service-base";
import { isRollbackableDeployStatus } from "@/features/deploys/lib/deploy-status";
import { ConfirmGateStatus } from "@/features/capabilities/components/confirm-gate-status";
import { useDeployActions } from "@/features/capabilities/hooks/use-resource-actions";
import { useBoundActionConfirm } from "@/features/capabilities/hooks/use-bound-action-confirm";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useAskForProtectedConfirmation } from "@/common/providers/protected-retry-context";
import {
  ProtectedConfirmationDismissed,
  withProtectedRetry,
} from "@/features/services/lib/protected-confirmation";
import {
  decisionForSelectedRollback,
  gateAction,
  gateReason,
  resourceDecision,
  type ResourceActionDecision,
  type ResourceActionId,
} from "@/features/capabilities/lib/resource-actions";
import { RollbackComparisonBody } from "./rollback-comparison";

/** bex's rollbacks page, Render's "Learn more." target. */
const ROLLBACKS_DOCS_URL = "https://bex.co/docs/platform/rollbacks";

export interface DeployRollbackProps {
  serviceId: string;
  deployId: string;
  /** The deploy's status; undefined while it loads. */
  status?: string;
  /** False when the registry no longer holds its build (w8/m156). */
  buildAvailable?: boolean | null;
  /** The service's auto-deploy is on, so the rollback turns it off. */
  autoDeployOn?: boolean;
}

/**
 * Render's Confirm Rollback page section (w1/m646,
 * docs/render-artifacts/rollback-comparison.md): on the deploy page's
 * `/rollback` route, in place of its log, the configuration the rollback
 * changes, then the notices, then "Rollback to this deploy" and Cancel. The
 * page stands for the open confirmation: it binds the rollback while shown
 * and rechecks the grant before dispatching, as the dialog it replaces did
 * (w4/m159), so a selected eligible deploy stays a target when the
 * service-wide summary found none (w6/m143/t003).
 */
export function DeployRollback({
  serviceId,
  deployId,
  status,
  buildAvailable,
  autoDeployOn,
}: DeployRollbackProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const base = useServiceBase();
  const staticSite = base === "/static";
  const titleId = useId();
  const { currentWorkspaceId } = useWorkspace();
  const deployActions = useDeployActions(serviceId);
  const rollbackable =
    status === undefined
      ? undefined
      : isRollbackableDeployStatus(status) && buildAvailable !== false;

  const deployPage = useCallback(
    (replace = false) =>
      void navigate({
        to: `${base}/$serviceId/deploys/$deployId`,
        params: { serviceId, deployId },
        replace,
      }),
    [navigate, base, serviceId, deployId],
  );
  // Render sends a deploy it cannot roll back to back to its own page.
  useEffect(() => {
    if (rollbackable === false) deployPage(true);
  }, [rollbackable, deployPage]);

  const rowDecision = useCallback(
    (
      action: ResourceActionId,
      decision: ResourceActionDecision | null,
    ): ResourceActionDecision | null =>
      action === "rollback" ? decisionForSelectedRollback(decision) : decision,
    [],
  );
  const {
    pending,
    blockedReason,
    openConfirm,
    clearConfirm,
    recheckBeforeDispatch,
    isIntentCurrent,
  } = useBoundActionConfirm({
    resourceId: serviceId,
    deployId,
    adjustDecision: rowDecision,
  });
  const reason =
    deployActions.status !== "ready"
      ? gateReason(gateAction(null, deployActions.status), t)
      : gateReason(
          gateAction(
            rowDecision(
              "rollback",
              resourceDecision(
                deployActions.snapshot,
                currentWorkspaceId,
                serviceId,
                "rollback",
              ),
            ),
            "ready",
          ),
          t,
        );
  // The page is the open confirmation: bind it whenever it may dispatch.
  useEffect(() => {
    if (rollbackable && !pending && !reason) openConfirm("rollback");
  }, [rollbackable, pending, reason, openConfirm]);

  const comparison = useQuery(RollbackComparisonDocument, {
    variables: { serviceId, deployId },
    fetchPolicy: "network-only",
    skip: !rollbackable,
  });
  const [rollbackService, { loading: rollingBack }] = useMutation(
    RollbackServiceDocument,
    { refetchQueries: DEPLOY_REFETCH_QUERIES },
  );
  const askForConfirmation = useAskForProtectedConfirmation();
  const [checking, setChecking] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const busy = checking || rollingBack;
  const blocked = blockedReason ?? reason;

  async function confirm() {
    if (busy || blocked || !rollbackable) return;
    if (!pending && !openConfirm("rollback")) return;
    setChecking(true);
    setFailure(null);
    try {
      const { ok, binding } = await recheckBeforeDispatch();
      if (!ok || !binding) return;
      // A rollback swaps the running code: a protected member answers with
      // the phrase to retry with (w4/m176).
      const { data } = await withProtectedRetry(askForConfirmation, (phrase) =>
        rollbackService({
          variables: {
            serviceId: binding.resourceId,
            deployId: binding.deployId ?? deployId,
            confirm: phrase,
          },
        }),
      );
      if (!isIntentCurrent(binding)) return;
      const rollbackId = data?.rollbackService?.id;
      if (!rollbackId) throw new Error("rollbackService returned no deploy id");
      clearConfirm();
      toast.success(t("services.rollbackSuccess"));
      void navigate({
        to: `${base}/$serviceId/deploys/$deployId`,
        params: { serviceId, deployId: rollbackId },
      });
    } catch (err) {
      if (err instanceof ProtectedConfirmationDismissed) return;
      setFailure(mutationErrorMessage(err, t("services.rollbackError")));
    } finally {
      setChecking(false);
    }
  }

  const [noticeBefore, noticeAfter] = aroundPlaceholder(
    t("deploys.rollbackStaticNotice", { what: PLACEHOLDER_SENTINEL }),
  );

  return (
    <section aria-labelledby={titleId} className="space-y-4">
      <div className="rounded-md border bg-card">
        <div className="space-y-1 border-b px-5 py-4">
          <h2 id={titleId} className="text-base font-semibold">
            {t("deploys.rollbackTitle")}
          </h2>
          <p className="text-sm text-muted-foreground">
            {t("deploys.rollbackLead")}{" "}
            <a
              href={ROLLBACKS_DOCS_URL}
              target="_blank"
              rel="noreferrer"
              className="font-medium text-foreground underline underline-offset-4"
            >
              {t("deploys.rollbackLearnMore")}
            </a>
          </p>
        </div>
        <RollbackComparisonBody
          loading={comparison.loading || rollbackable === undefined}
          error={!!comparison.error}
          data={comparison.data?.rollbackComparison}
        />
      </div>
      {staticSite || autoDeployOn ? (
        <div className="space-y-2">
          {staticSite ? (
            <Alert role="note">
              <Info />
              <AlertDescription>
                <p>
                  {noticeBefore}
                  <span className="font-semibold text-foreground">
                    {t("deploys.rollbackStaticNoticeWhat")}
                  </span>
                  {noticeAfter}
                </p>
              </AlertDescription>
            </Alert>
          ) : null}
          {autoDeployOn ? (
            <Alert
              role="note"
              className="border-amber-500/40 text-amber-800 dark:text-amber-300"
            >
              <TriangleAlert />
              <AlertTitle>{t("deploys.rollbackAutoDeployTitle")}</AlertTitle>
              <AlertDescription>
                <p>{t("deploys.rollbackAutoDeployBody")}</p>
              </AlertDescription>
            </Alert>
          ) : null}
        </div>
      ) : null}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          onClick={() => void confirm()}
          disabled={busy || !!blocked || !rollbackable}
        >
          {busy ? (
            <Loader2
              aria-hidden
              className="animate-spin motion-reduce:animate-none"
            />
          ) : null}
          {t("deploys.rollbackConfirm")}
        </Button>
        <Button variant="outline" disabled={busy} onClick={() => deployPage()}>
          {t("deploys.rollbackCancel")}
        </Button>
      </div>
      <ConfirmGateStatus reason={blocked} />
      {failure ? (
        <p role="alert" className="text-sm text-destructive">
          <span className="font-medium">{t("deploys.rollbackErrorTitle")}</span>{" "}
          {failure}
        </p>
      ) : null}
    </section>
  );
}
