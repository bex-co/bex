import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetEnvironmentServicesDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseSetEnvironmentServicesResult {
  /**
   * Full-replace an environment's service membership (bex-api's
   * `setEnvironmentServices`). Assigning services auto-joins them to the
   * environment's parent project (docs/ADR032-environments.md). Resolves true
   * on success (toasted either way).
   */
  setServices: (
    id: string,
    envName: string,
    serviceIds: string[],
  ) => Promise<boolean>;
  /** The environment id currently being written, or null. */
  busyId: string | null;
}

/**
 * Wires the environment "Manage services" dialog to bex-api's
 * `setEnvironmentServices` — a full-replace verb, so the dialog computes the
 * wanted set client-side from its checkboxes (checked = assigned, unchecked =
 * removed) and hands it the complete list. That single verb covers both
 * assigning and removing a service.
 */
export function useSetEnvironmentServices(): UseSetEnvironmentServicesResult {
  const { t } = useTranslations();
  // Refetch the Environments list (new membership) AND the Projects list —
  // assigning a service auto-joins it to the parent project, so the project's
  // own resource table must reflect the new serviceIds too. Refetch by name so
  // no caller has to thread a refetch callback down (docs/ADR032-environments.md).
  const [mutate] = useMutation(SetEnvironmentServicesDocument, {
    refetchQueries: ["Environments", "Projects"],
    awaitRefetchQueries: true,
  });
  const [busyId, setBusyId] = useState<string | null>(null);

  const setServices = useCallback(
    async (id: string, envName: string, serviceIds: string[]) => {
      setBusyId(id);
      try {
        await mutate({ variables: { id, serviceIds } });
        toast.success(t("environments.assignSuccess", { name: envName }));
        return true;
      } catch {
        toast.error(t("environments.assignError", { name: envName }));
        return false;
      } finally {
        setBusyId(null);
      }
    },
    [mutate, t],
  );

  return { setServices, busyId };
}
