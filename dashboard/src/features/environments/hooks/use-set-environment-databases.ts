import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetEnvironmentDatabasesDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseSetEnvironmentDatabasesResult {
  /**
   * Full-replace an environment's database membership (bex-api's
   * `setEnvironmentDatabases`, w6/m20 extension). Assigning databases
   * auto-joins them to the environment's parent project, mirroring
   * useSetEnvironmentServices. Resolves true on success (toasted either way).
   */
  setDatabases: (
    id: string,
    envName: string,
    databaseIds: string[],
  ) => Promise<boolean>;
  /** The environment id currently being written, or null. */
  busyId: string | null;
}

/**
 * Wires the environment "Manage resources" dialog's Databases tab to
 * bex-api's `setEnvironmentDatabases` — a full-replace verb, mirroring
 * useSetEnvironmentServices exactly (see that hook's doc comment).
 */
export function useSetEnvironmentDatabases(): UseSetEnvironmentDatabasesResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetEnvironmentDatabasesDocument, {
    refetchQueries: ["Environments", "Projects"],
    awaitRefetchQueries: true,
  });
  const [busyId, setBusyId] = useState<string | null>(null);

  const setDatabases = useCallback(
    async (id: string, envName: string, databaseIds: string[]) => {
      setBusyId(id);
      try {
        await mutate({ variables: { id, databaseIds } });
        toast.success(
          t("environments.assignDatabasesSuccess", { name: envName }),
        );
        return true;
      } catch {
        toast.error(t("environments.assignDatabasesError", { name: envName }));
        return false;
      } finally {
        setBusyId(null);
      }
    },
    [mutate, t],
  );

  return { setDatabases, busyId };
}
