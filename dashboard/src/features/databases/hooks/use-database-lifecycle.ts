import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SuspendDatabaseDocument,
  ResumeDatabaseDocument,
  RestartDatabaseDocument,
} from "@/features/databases/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";
import type { DatabaseView } from "@/features/databases/types";
import {
  protectedConfirmationFromError,
  type ProtectedActionResult,
} from "@/features/services/lib/protected-confirmation";

/** The managed-Postgres lifecycle verbs (mirrors the compute lifecycle). */
export type DatabaseLifecycleAction = "suspend" | "resume" | "restart";

/** The action currently in flight for a given database id (or null). */
export interface PendingDatabaseLifecycle {
  id: string;
  action: DatabaseLifecycleAction;
}

export interface UseDatabaseLifecycleOptions {
  /** Re-reads the database so the header reflects the operator's new state. */
  refetch: () => void;
}

export interface UseDatabaseLifecycleResult {
  pending: PendingDatabaseLifecycle | null;
  run: (
    action: DatabaseLifecycleAction,
    database: DatabaseView,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
}

const SUCCESS_KEY: Record<DatabaseLifecycleAction, string> = {
  suspend: "databases.toastSuspendSuccess",
  resume: "databases.toastResumeSuccess",
  restart: "databases.toastRestartSuccess",
};

/**
 * Wires the database detail header's lifecycle actions to bex-api's mutations
 * (`suspendDatabase`/`resumeDatabase`/`restartDatabase`). The patch is accepted
 * synchronously (the spec flips) but the operator hibernates/restarts CNPG
 * asynchronously, so after each verb we refetch the detail — the header reflects
 * the operator, not an optimistic guess. Mirrors useServiceLifecycle.
 */
export function useDatabaseLifecycle(
  opts: UseDatabaseLifecycleOptions,
): UseDatabaseLifecycleResult {
  const { refetch } = opts;
  const { t } = useTranslations();
  const [pending, setPending] = useState<PendingDatabaseLifecycle | null>(null);

  const [suspend] = useMutation(SuspendDatabaseDocument);
  const [resume] = useMutation(ResumeDatabaseDocument);
  const [restart] = useMutation(RestartDatabaseDocument);
  const mutate: Record<
    DatabaseLifecycleAction,
    (id: string, confirmation?: string) => Promise<unknown>
  > = {
    suspend: (id, confirmation) =>
      suspend({ variables: { id, confirm: confirmation } }),
    resume: (id) => resume({ variables: { id } }),
    restart: (id) => restart({ variables: { id } }),
  };

  const run = useCallback(
    async (
      action: DatabaseLifecycleAction,
      database: DatabaseView,
      confirmation?: string,
    ) => {
      setPending({ id: database.id, action });
      try {
        await mutate[action](database.id, confirmation);
        toast.success(t(SUCCESS_KEY[action], { name: database.name }));
        refetch();
        return { status: "success" } as const;
      } catch (err) {
        const requiredConfirmation = protectedConfirmationFromError(err);
        if (requiredConfirmation) {
          return {
            status: "confirmation_required",
            confirmation: requiredConfirmation,
          } as const;
        }
        toast.error(
          t("databases.toastLifecycleError", { name: database.name }),
        );
        return { status: "error" } as const;
      } finally {
        setPending(null);
      }
    },
    // mutate closes over the three stable mutate fns; t/refetch are stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, refetch],
  );

  return { pending, run };
}
