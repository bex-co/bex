import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { TriggerDeployDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

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
   * bex builds are always cache-free (ephemeral BuildKit Jobs), so "clear" is a
   * no-op — every trigger already rebuilds from a clean slate. Sent for
   * REST/GraphQL/MCP parity; the backend enum-validates but does not branch on it.
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
}

/**
 * Render's header-level "Manual Deploy" verb. It lives in the service header
 * (not on the Events tab), but the Events list and the deploy history are what
 * show its result, so the mutation refetches exactly those queries after
 * success — Server (header chrome/status), Deploys (history), ServiceEvents —
 * rather than every active query (which refetched 6-10 queries per trigger,
 * including unrelated polling lists). Named queries match only mounted
 * instances, so nothing inactive is refetched.
 *
 * Also used for "Restart service" (w2/m30 consolidation): passing no opts
 * triggers a rebuild for repo-backed services and a pure restart for
 * image-backed ones — both paths open a deploy-history row.
 */
export function useTriggerDeploy(): UseTriggerDeployResult {
  const { t } = useTranslations();
  const [triggerDeploy, { loading }] = useMutation(TriggerDeployDocument, {
    refetchQueries: ["Server", "Deploys", "ServiceEvents"],
    awaitRefetchQueries: true,
  });

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
    } catch {
      toast.error(t("services.triggerDeployError"));
      return null;
    }
  }

  return { deploying: loading, trigger };
}
