import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  RestartServerDocument,
  TriggerDeployDocument,
} from "@/graphql/definitions";
import { DEPLOY_REFETCH_QUERIES } from "@/common/lib/fetch-policy";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

const REFETCH_DEPLOYS = {
  refetchQueries: DEPLOY_REFETCH_QUERIES,
  awaitRefetchQueries: true,
};

export interface TriggerOptions {
  /** Pin the build to a specific Git ref instead of Branch HEAD. Repo-backed only. */
  commitId?: string;
  /**
   * "deploy_only" skips the build step — valid only for image-backed services.
   * Repo-backed services reject this with an error (bex has no cached build
   * artifact; any trigger unconditionally rebuilds from source).
   * Omit or pass "build_and_deploy" for the normal full-rebuild path.
   */
  deployMode?: string;
  /**
   * Render's "clear" | "do_not_clear" enum ("Clear build cache and deploy").
   * With BEX_BUILD_CACHE=registry, "clear" rebuilds without importing prior
   * layers and still exports a fresh cache (w7/m88). With the gate off both
   * values are no-ops (ephemeral Jobs start empty).
   */
  clearCache?: string;
}

export interface UseTriggerDeployResult {
  /** True while the deploy mutation (and its Events refetch) is in flight. */
  deploying: boolean;
  /**
   * Trigger a manual deploy of `serviceId`, toasting the outcome. Resolves the
   * new deploy's id on success (w9/m1/t004 — callers navigate to its page), or
   * null on failure (the toast already reported it; the caller shouldn't also
   * navigate).
   */
  trigger: (serviceId: string, opts?: TriggerOptions) => Promise<string | null>;
  /**
   * Restart `serviceId` on the commit or image it is running (`restartServer`,
   * w1/m148), toasting the outcome. Resolves the deploy id it opens, or null
   * on failure — the same contract as `trigger`.
   */
  restart: (serviceId: string) => Promise<string | null>;
}

/**
 * Render's header-level "Manual Deploy" verb. It lives in the service header
 * (not on the Events tab), but the Events list and the deploy history are what
 * show its result, so the mutation refetches exactly DEPLOY_REFETCH_QUERIES
 * after success rather than every active query (which refetched 6-10 queries
 * per trigger, including unrelated polling lists).
 *
 * "Restart service" is a separate mutation, not a parameter-free trigger: a
 * trigger builds the branch head, while a restart keeps the running commit
 * (w1/m148). Both open a deploy-history row.
 */
export function useTriggerDeploy(): UseTriggerDeployResult {
  const { t } = useTranslations();
  const [triggerDeploy, { loading }] = useMutation(
    TriggerDeployDocument,
    REFETCH_DEPLOYS,
  );
  const [restartServer, { loading: restarting }] = useMutation(
    RestartServerDocument,
    REFETCH_DEPLOYS,
  );

  async function restart(serviceId: string): Promise<string | null> {
    try {
      const { data } = await restartServer({ variables: { serviceId } });
      toast.success(t("services.restartServiceSuccess"));
      return data?.restartServer?.id ?? null;
    } catch (err) {
      toast.error(mutationErrorMessage(err, t("services.restartServiceError")));
      return null;
    }
  }

  async function trigger(
    serviceId: string,
    opts?: TriggerOptions,
  ): Promise<string | null> {
    try {
      const { data } = await triggerDeploy({
        variables: {
          serviceId,
          commitId: opts?.commitId,
          deployMode: opts?.deployMode,
          clearCache: opts?.clearCache,
        },
      });
      toast.success(t("services.triggerDeploySuccess"));
      return data?.triggerDeploy?.id ?? null;
    } catch (err) {
      toast.error(mutationErrorMessage(err, t("services.triggerDeployError")));
      return null;
    }
  }

  return { deploying: loading || restarting, trigger, restart };
}
