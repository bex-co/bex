import { useCallback, useState } from "react";
import type { ApolloClient } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  ServerDocument,
  SetStartCommandDocument,
  UpdateCronJobDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useAskForProtectedConfirmation } from "@/common/providers/protected-retry-context";
import {
  ProtectedConfirmationDismissed,
  withProtectedRetry,
} from "@/features/services/lib/protected-confirmation";

export interface UseCronJobResult {
  /**
   * Fires updateCronJob; resolves true on success (toasted either way).
   *
   * `command` carries three distinct intents and all three reach the wire
   * unchanged, because the backend distinguishes them: `null` keeps the stored
   * command, a nonempty string replaces it, and an explicit `""` **clears** the
   * override so the job runs its image's own command (`apps/service.go`'s
   * nil-means-keep / empty-means-clear contract, and the field's own hint says
   * to leave it blank for exactly that).
   *
   * This used to send `command || null`, which collapsed the third intent into
   * the first: clearing the field saved successfully, preserved the old
   * command, and toasted success (w4/137).
   */
  updateCronJob: (
    id: string,
    schedule: string,
    command: string | null,
  ) => Promise<boolean>;
  /** A native cron's Command save (w8/m69), refreshed like updateCronJob. */
  setStartCommand: (id: string, command: string) => Promise<boolean>;
  busy: boolean;
  /**
   * The service whose save committed but whose authoritative re-read has not
   * published yet (its refresh failed), or null. That service's rows must
   * stay unwritable: their displayed values may be the pre-save snapshot a
   * late read restored, and a sibling-field save would replay it (w4/276).
   */
  staleId: string | null;
}

/**
 * Publish a committed cron save (w4/276), following the rename ordering in
 * use-rename-project: settle this service's in-flight Server reads first —
 * refetchQueries dedupes onto a read already in flight, and each result is
 * settled on its own because the aggregate rejects on the first failure
 * while others may still be writing the pre-save snapshot — and only then
 * re-read by id. queryDeduplication:false keeps that last read from joining
 * an older in-flight one. Whatever it returns is authoritative, including a
 * newer configuration another actor saved in the meantime.
 */
async function refreshCronServer(client: ApolloClient, id: string) {
  const reads = client.refetchQueries({
    include: [ServerDocument],
    onQueryUpdated: (query) =>
      query.variables.id === id && query.getCurrentResult().loading,
  });
  reads.catch(() => undefined); // each result is settled below
  await Promise.allSettled(reads.results);
  await client.query({
    query: ServerDocument,
    variables: { id },
    fetchPolicy: "network-only",
    context: { queryDeduplication: false },
  });
}

/**
 * Wires the cron Deploy section's Schedule + Command controls to bex-api's
 * `updateCronJob` mutation (w5/m18), and a native cron's Command to
 * `setStartCommand` (w8/m69). updateCronJob patches spec.schedule and
 * spec.command; the operator applies the new schedule to the k8s CronJob on
 * its next reconcile pass. The toast confirms the write rather than implying
 * instant convergence.
 */
export function useCronJob(): UseCronJobResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutateCron] = useMutation(UpdateCronJobDocument);
  const [mutateStartCommand] = useMutation(SetStartCommandDocument);
  const [busy, setBusy] = useState(false);
  const [staleId, setStaleId] = useState<string | null>(null);
  const askForConfirmation = useAskForProtectedConfirmation();

  // The write already committed: a failed refresh is never a failed save,
  // and its retry re-reads without saving again. Until it succeeds the
  // service stays staleId-blocked so no save can replay pre-save values.
  const publish = useCallback(
    (id: string, converging?: string): Promise<void> => {
      const attempt = async () => {
        try {
          await refreshCronServer(client, id);
        } catch {
          setStaleId(id);
          toast.warning(t("services.deployRefreshFailed"), {
            action: {
              label: t("common.tryAgain"),
              onClick: () => void attempt(),
            },
          });
          return;
        }
        setStaleId((prev) => (prev === id ? null : prev));
        toast.success(t("services.deploySuccess"), {
          description: converging,
        });
      };
      return attempt();
    },
    [client, t],
  );

  // One cron write: refused while this service's last save is unpublished,
  // retried through a protected environment's confirmation, then published.
  const save = useCallback(
    async (
      id: string,
      write: (confirm?: string) => Promise<unknown>,
      converging?: string,
    ) => {
      // A previous save for this service committed but never published:
      // the caller's sibling value may be the restored pre-save snapshot.
      if (staleId === id) return false;
      setBusy(true);
      try {
        try {
          await withProtectedRetry(askForConfirmation, write);
        } catch (err) {
          if (err instanceof ProtectedConfirmationDismissed) return false;
          toast.error(mutationErrorMessage(err, t("services.deployError")));
          return false;
        }
        await publish(id, converging);
        return true;
      } finally {
        setBusy(false);
      }
    },
    [publish, staleId, t, askForConfirmation],
  );

  // Changing the command changes what the job runs, which a protected
  // environment guards; rescheduling alone does not (w4/m126).
  const updateCronJob = useCallback(
    (id: string, schedule: string, command: string | null) =>
      save(
        id,
        (confirm) =>
          mutateCron({ variables: { id, schedule, command, confirm } }),
        t("services.deployConverging"),
      ),
    [mutateCron, save, t],
  );

  const setStartCommand = useCallback(
    (id: string, command: string) =>
      save(id, (confirm) =>
        mutateStartCommand({ variables: { id, command, confirm } }),
      ),
    [mutateStartCommand, save],
  );

  return { updateCronJob, setStartCommand, busy, staleId };
}
