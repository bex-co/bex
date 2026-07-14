import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { TriggerDeployDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseTriggerDeployResult {
  /** True while the deploy mutation (and its Events refetch) is in flight. */
  deploying: boolean;
  /** Trigger a manual deploy of `serviceId`, toasting the outcome. */
  trigger: (serviceId: string) => Promise<void>;
}

/**
 * Render's header-level "Manual Deploy" verb. It lives in the service header
 * (not on the Events tab), but the Events list is what shows its result, so the
 * mutation refetches the active `ServiceEvents` query by name — the new deploy
 * event then appears without the header having to know the tab's variables.
 */
export function useTriggerDeploy(): UseTriggerDeployResult {
  const { t } = useTranslations();
  const [triggerDeploy, { loading }] = useMutation(TriggerDeployDocument, {
    refetchQueries: ["ServiceEvents"],
    awaitRefetchQueries: true,
  });

  async function trigger(serviceId: string) {
    try {
      await triggerDeploy({ variables: { serviceId } });
      toast.success(t("services.triggerDeploySuccess"));
    } catch {
      toast.error(t("services.triggerDeployError"));
    }
  }

  return { deploying: loading, trigger };
}
