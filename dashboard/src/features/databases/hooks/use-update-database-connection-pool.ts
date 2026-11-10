import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateDatabaseConnectionPoolDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseUpdateDatabaseConnectionPoolResult {
  /** Turns PgBouncer on (`pgbouncer`) or off (`none`); true when accepted. */
  updateConnectionPool: (
    id: string,
    name: string,
    enabled: boolean,
  ) => Promise<boolean>;
  busy: boolean;
}

/**
 * Render's Info > Connection Pool toggle (w2/m307): REST's PATCH
 * connectionPool through GraphQL. The answer carries `poolerEnabled`, so the
 * cached database flips without waiting for a refetch.
 */
export function useUpdateDatabaseConnectionPool(): UseUpdateDatabaseConnectionPoolResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateDatabaseConnectionPoolDocument);
  const [busy, setBusy] = useState(false);

  const updateConnectionPool = useCallback(
    async (id: string, name: string, enabled: boolean) => {
      setBusy(true);
      try {
        await mutate({
          variables: { id, connectionPool: enabled ? "pgbouncer" : "none" },
        });
        toast.success(
          t(
            enabled
              ? "databases.poolerEnableSuccess"
              : "databases.poolerDisableSuccess",
            { name },
          ),
        );
        return true;
      } catch (err) {
        toast.error(
          mutationErrorMessage(
            err,
            t(
              enabled
                ? "databases.poolerEnableError"
                : "databases.poolerDisableError",
              { name },
            ),
          ),
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { updateConnectionPool, busy };
}
