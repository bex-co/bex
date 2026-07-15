import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetEnvironmentKeyValuesDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseSetEnvironmentKeyValuesResult {
  /**
   * Full-replace an environment's key-value membership (bex-api's
   * `setEnvironmentKeyValues`, w6/m20 extension). Assigning key-value
   * instances auto-joins them to the environment's parent project, mirroring
   * useSetEnvironmentDatabases. Resolves true on success (toasted either way).
   */
  setKeyValues: (
    id: string,
    envName: string,
    keyValueIds: string[],
  ) => Promise<boolean>;
  /** The environment id currently being written, or null. */
  busyId: string | null;
}

/**
 * Wires the environment "Manage resources" dialog's Key Value tab to
 * bex-api's `setEnvironmentKeyValues` — a full-replace verb, mirroring
 * useSetEnvironmentServices/useSetEnvironmentDatabases exactly.
 */
export function useSetEnvironmentKeyValues(): UseSetEnvironmentKeyValuesResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetEnvironmentKeyValuesDocument, {
    refetchQueries: ["Environments", "Projects"],
    awaitRefetchQueries: true,
  });
  const [busyId, setBusyId] = useState<string | null>(null);

  const setKeyValues = useCallback(
    async (id: string, envName: string, keyValueIds: string[]) => {
      setBusyId(id);
      try {
        await mutate({ variables: { id, keyValueIds } });
        toast.success(
          t("environments.assignKeyValuesSuccess", { name: envName }),
        );
        return true;
      } catch {
        toast.error(t("environments.assignKeyValuesError", { name: envName }));
        return false;
      } finally {
        setBusyId(null);
      }
    },
    [mutate, t],
  );

  return { setKeyValues, busyId };
}
