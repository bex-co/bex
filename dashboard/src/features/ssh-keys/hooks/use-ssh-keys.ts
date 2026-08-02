import { useCallback, useMemo, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CreateSshKeyDocument,
  DeleteSshKeyDocument,
  SshKeysDocument,
} from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useTranslations } from "@/common/hooks/use-translations";
import type { SSHKeyView } from "@/features/ssh-keys/types";

export function useSSHKeys() {
  const { t } = useTranslations();
  const { data, loading, error, refetch } = useQuery(SshKeysDocument, {
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const [createMutation] = useMutation(CreateSshKeyDocument);
  const [deleteMutation] = useMutation(DeleteSshKeyDocument);
  const [busy, setBusy] = useState<string | null>(null);

  const keys = useMemo<SSHKeyView[]>(
    () =>
      (data?.sshKeys ?? [])
        .filter(
          (key): key is NonNullable<typeof key> => key != null && !!key.id,
        )
        .map((key) => ({
          id: key.id,
          name: key.name,
          publicKey: key.publicKey,
          fingerprint: key.fingerprint,
          createdAt: key.createdAt,
        })),
    [data],
  );

  const create = useCallback(
    async (name: string, publicKey: string) => {
      setBusy("create");
      try {
        await createMutation({ variables: { name, publicKey } });
        await refetch();
        toast.success(t("sshKeys.createSuccess", { name }));
        return true;
      } catch (cause) {
        const duplicate =
          cause instanceof Error &&
          cause.message.toLowerCase().includes("already registered");
        toast.error(
          t(duplicate ? "sshKeys.duplicateError" : "sshKeys.createError"),
        );
        return false;
      } finally {
        setBusy(null);
      }
    },
    [createMutation, refetch, t],
  );

  const remove = useCallback(
    async (id: string, name: string) => {
      setBusy(id);
      try {
        await deleteMutation({ variables: { id } });
        await refetch();
        toast.success(t("sshKeys.deleteSuccess", { name }));
        return true;
      } catch {
        toast.error(t("sshKeys.deleteError"));
        return false;
      } finally {
        setBusy(null);
      }
    },
    [deleteMutation, refetch, t],
  );

  return { keys, loading, error, busy, create, remove };
}
