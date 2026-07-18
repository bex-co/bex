import { useEffect } from "react";
import { useQuery } from "@apollo/client/react";
import { DeployDocument, type DeployQuery } from "@/graphql/definitions";
import { isTerminalDeployStatus } from "@/features/deploys/lib/deploy-status";

/** How often to re-poll a non-terminal deploy (build/update in flight). */
const POLL_INTERVAL_MS = 3000;

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
}

export interface UseDeployResult {
  deploy: DeployView | null;
  /** True only on the first load, before any deploy has been resolved. */
  loading: boolean;
  error: Error | undefined;
  /** True once bex-api resolved the query and found no such deploy. */
  notFound: boolean;
}

/**
 * Reads bex-api's `deploy(serviceId, deployId)` (w9/m1/t001) — the deploy
 * detail page's header data. Polls every 3s while the deploy is non-terminal
 * so the header/log window follows each observed phase, and stops the moment
 * it reaches any terminal status.
 */
export function useDeploy(
  serviceId: string,
  deployId: string,
): UseDeployResult {
  const { data, loading, error, previousData, stopPolling } = useQuery(
    DeployDocument,
    {
      variables: { serviceId, deployId },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      pollInterval: POLL_INTERVAL_MS,
    },
  );

  const deploy = toDeployView(data?.deploy ?? previousData?.deploy ?? null);
  const notFound =
    !loading &&
    !deploy &&
    (!error || error.message.toLowerCase().includes("not found"));

  useEffect(() => {
    if (deploy && isTerminalDeployStatus(deploy.status)) stopPolling();
  }, [deploy, stopPolling]);

  return {
    deploy,
    loading: loading && !deploy,
    error: notFound ? undefined : error,
    notFound,
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
  };
}
