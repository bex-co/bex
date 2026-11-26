import {
  DeployDocument,
  DeploysDocument,
  ServerDocument,
  WorkspaceOverlappingDeployPolicyDocument,
} from "@/graphql/definitions";
import { prefetchInParallel } from "@/common/lib/prefetch";
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
