import { useEffect } from "react";
import { useQuery } from "@apollo/client/react";
import { resourceNotFound } from "@/common/hooks/use-not-found-redirect";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { DeployDocument, type DeployQuery } from "@/graphql/definitions";
import { isTerminalDeployStatus } from "@/features/deploys/lib/deploy-status";

/** How often to re-poll a non-terminal deploy (build/update in flight). */
const POLL_INTERVAL_MS = 3000;

/**
 * How long after a cancellation to keep polling for the verdict of a
 * pre-deploy step that may still be running (w4/m198): the step's Job has a
 * 10-minute deadline (operator internal/predeploy), plus reconcile slack. A
 * step that was actually stopped keeps reading "canceled", so this bound — not
 * the verdict — is what ends the poll for it.
 */
const STEP_SETTLE_MS = 12 * 60 * 1000;

export interface DeployView {
  id: string;
  status: string;
  trigger: string;
  image: string;
  rollbackOf: string;
  /** The resolved commit this deploy ran (w9/001 + w2/m42); "" when unresolved. */
  commitId: string;
  commitMessage: string;
  /** Git author timestamp of the commit (w2/m42); null when unavailable. */
  commitCreatedAt: string | null;
  createdAt: string | null;
  updatedAt: string | null;
  startedAt: string | null;
  finishedAt: string | null;
  preDeployStatus: string;
  /** Actionable cause of a failed deploy (w9/011); "" unless it failed. */
  failureReason: string;
  /** Neutral cause of a non-user cancel (w4/089); "" unless superseded. */
  cancelReason: string;
  /**
   * Why an OPEN deploy is not progressing (w4/m112) — the operator's live
   * diagnosis of the current revision's pods, e.g. a failing health check
   * naming its path. "" while a rollout is healthy, and cleared by the server
   * the moment the deploy goes terminal. An observation, not a verdict.
   */
  stallReason: string;
}

export interface UseDeployResult {
  deploy: DeployView | null;
  /** True only on the first load, before any deploy has been resolved. */
  loading: boolean;
  error: Error | undefined;
  /** True once bex-api resolved the query and found no such deploy. */
  notFound: boolean;
  /** Re-read the deploy, e.g. to retry after a failed refresh. */
  refetch: () => void;
}

/**
 * Reads bex-api's `deploy(serviceId, deployId)` (w9/m1/t001) — the deploy
 * detail page's header data. Polls every 3s while the deploy is non-terminal
 * so the header/log window follows each observed phase, and stops once it is
 * terminal — except a canceled deploy whose pre-deploy step may still settle
 * its verdict, which is read for that step's bounded lifetime (w4/m198), and a
 * Live deploy, which a later deploy will replace (w4/262).
 */
export function useDeploy(
  serviceId: string,
  deployId: string,
): UseDeployResult {
  const {
    data,
    loading,
    error,
    previousData,
    startPolling,
    stopPolling,
    refetch,
  } = useQuery(DeployDocument, {
    variables: { serviceId, deployId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const deploy = toDeployView(data?.deploy ?? previousData?.deploy ?? null);
  const notFound = resourceNotFound(deploy, loading, error);

  const status = deploy?.status ?? "";
  const preDeployStatus = deploy?.preDeployStatus ?? "";
  const finishedAt = deploy?.finishedAt ?? null;
  useEffect(() => {
    if (!isTerminalDeployStatus(status)) return;
    // Live ends the rollout, not the record: the next deploy turns it
    // Deactivated (and so rollbackable), so keep reading at the baseline.
    if (status === "live") {
      startPolling(RESOURCE_POLL_INTERVAL_MS);
      return;
    }
    // Keep reading it even when this deploy is no longer the service's latest.
    if (status === "canceled" && preDeployStatus === "canceled" && finishedAt) {
      const remaining = Date.parse(finishedAt) + STEP_SETTLE_MS - Date.now();
      if (remaining > 0) {
        const timer = setTimeout(stopPolling, remaining);
        return () => clearTimeout(timer);
      }
    }
    stopPolling();
  }, [status, preDeployStatus, finishedAt, startPolling, stopPolling]);

  return {
    deploy,
    loading: loading && !deploy,
    error: notFound ? undefined : error,
    notFound,
    refetch: () => void refetch(),
  };
}

function toDeployView(
  d: DeployQuery["deploy"] | null | undefined,
): DeployView | null {
  if (!d || !d.id) return null;
  return {
    id: d.id,
    status: d.status ?? "",
    trigger: d.trigger ?? "",
    image: d.image ?? "",
    rollbackOf: d.rollbackOf ?? "",
    commitId: d.commitId ?? "",
    commitMessage: d.commitMessage ?? "",
    commitCreatedAt: d.commitCreatedAt ?? null,
    createdAt: d.createdAt ?? null,
    updatedAt: d.updatedAt ?? null,
    startedAt: d.startedAt ?? null,
    finishedAt: d.finishedAt ?? null,
    preDeployStatus: d.preDeployStatus ?? "",
    failureReason: d.failureReason ?? "",
    cancelReason: d.cancelReason ?? "",
    stallReason: d.stallReason ?? "",
  };
}
