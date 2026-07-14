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
 * (not on the Events tab), but the Events list is what shows its result, so the
 * mutation refetches the active `ServiceEvents` query by name — the new deploy
 * event then appears without the header having to know the tab's variables.
 *
 * Also used for "Restart service" (w2/m30 consolidation): passing no opts
 * triggers a rebuild for repo-backed services and a pure restart for
 * image-backed ones — both paths open a deploy-history row.
 */
export function useTriggerDeploy(): UseTriggerDeployResult {
  const { t } = useTranslations();
  const [triggerDeploy, { loading }] = useMutation(TriggerDeployDocument, {
    refetchQueries: ["ServiceEvents"],
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
