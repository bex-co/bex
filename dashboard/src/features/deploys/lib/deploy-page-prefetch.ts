import {
  DeployDocument,
  DeploysDocument,
  RollbackComparisonDocument,
  ServerDocument,
  WorkspaceOverlappingDeployPolicyDocument,
} from "@/graphql/definitions";
import { prefetchInParallel } from "@/common/lib/prefetch";
import { titleLoaderFetchPolicy } from "@/common/lib/document-head";
import type { RouterContext } from "@/common/types/router-context";
import { inProgressDeploysVariables, noticeReads } from "./deploy-notices";

/**
 * Warms a deploy page's reads from its route loader (prefetch.ts, w9/m68): the
 * deploy, then what its Notices read beyond it (w1/m659) — a queued deploy's
 * in-progress blocker and the workspace's Overlapping Deploy Policy — with the
 * variables the page's hooks use. Hover intent prefetches them, a direct
 * navigation waits on them, and the page mounts with its Notices drawn: no
 * frame has to guess whether a deploy has any and then grow by them.
 *
 * cache-first throughout: a deploy the session already read opens at once,
 * and the page's own cache-and-network reads refresh it on mount. Best effort:
 * a failed read is swallowed, and the page's hooks surface it.
 */
export async function prefetchDeployPage(
  client: RouterContext["client"],
  serviceId: string,
  deployId: string,
): Promise<void> {
  const [deploy, ownerId] = await Promise.all([
    client
      .query({
        query: DeployDocument,
        variables: { serviceId, deployId },
        fetchPolicy: "cache-first",
        errorPolicy: "all",
      })
      .then(
        (result) => result.data?.deploy ?? null,
        () => null,
      ),
    // The service layout's loader reads it too; the Notices read the
    // workspace it belongs to.
    client
      .query({
        query: ServerDocument,
        variables: { id: serviceId },
        fetchPolicy: "cache-first",
        errorPolicy: "all",
      })
      .then(
        (result) => result.data?.server?.ownerId ?? null,
        () => null,
      ),
  ]);
  if (!deploy) return;
  const reads = noticeReads(deploy);
  const thunks: Array<() => Promise<unknown>> = [];
  if (reads.blocker) {
    thunks.push(() =>
      client.query({
        query: DeploysDocument,
        variables: inProgressDeploysVariables(serviceId),
        fetchPolicy: "cache-first",
        errorPolicy: "all",
      }),
    );
  }
  if (reads.policy && ownerId) {
    thunks.push(() =>
      client.query({
        query: WorkspaceOverlappingDeployPolicyDocument,
        variables: { id: ownerId },
        fetchPolicy: "cache-first",
        errorPolicy: "all",
      }),
    );
  }
  await prefetchInParallel(thunks);
}

/**
 * Warms a deploy's Confirm Rollback page (w1/m660): the deploy page's reads,
 * and beside them the rollback comparison, which the page then reads from the
 * cache. The page mounts with its comparison drawn, rather than over the
 * comparison's spinner frame that then grows by the changes the rollback
 * reverts. The comparison is read fresh on entering or hovering the route,
 * since it compares against whatever is live now; a retained match answers
 * from the cache. Best effort, like the deploy page's reads.
 */
export async function prefetchRollbackPage(
  client: RouterContext["client"],
  serviceId: string,
  deployId: string,
  cause: "preload" | "enter" | "stay" = "enter",
): Promise<void> {
  await prefetchInParallel([
    () => prefetchDeployPage(client, serviceId, deployId),
    () =>
      client.query({
        query: RollbackComparisonDocument,
        variables: { serviceId, deployId },
        fetchPolicy: titleLoaderFetchPolicy(cause),
        errorPolicy: "all",
      }),
  ]);
}
