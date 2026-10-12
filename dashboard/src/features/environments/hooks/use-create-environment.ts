import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateEnvironmentDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { publishEnvironments } from "@/features/environments/lib/refresh-environments";

export interface UseCreateEnvironmentResult {
  /**
   * Fires createEnvironment exactly once. Resolves the new id when the server
   * created it — even if the follow-up list refresh failed, which is reported
   * separately and never retries the write — and null when it refused.
   */
  create: (name: string) => Promise<string | null>;
  /** True from submit until the new environment is listed (or its refresh failed). */
  busy: boolean;
}

/**
 * Wires the "New Environment" dialog to bex-api's `createEnvironment` (w1/m32,
 * bex extension). Scoped to a project — the workspace is inherited from the
 * project server-side, so the caller only supplies the name.
 */
export function useCreateEnvironment(
  projectId: string,
): UseCreateEnvironmentResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [mutate] = useMutation(CreateEnvironmentDocument);
  const [busy, setBusy] = useState(false);

  const publish = useCallback(
    (name: string) =>
      publishEnvironments(client, projectId, {
        succeeded: t("environments.createSuccess", { name }),
        refreshFailed: t("environments.createRefreshFailed", { name }),
        tryAgain: t("common.tryAgain"),
      }),
    [client, projectId, t],
  );

  const create = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        let id: string | null | undefined;
        try {
          const res = await mutate({ variables: { name, projectId } });
          id = res.data?.createEnvironment?.id;
          if (!id) throw new Error("createEnvironment returned no id");
        } catch (err) {
          toast.error(
            mutationErrorMessage(err, t("environments.createError", { name })),
          );
          return null;
        }
        await publish(name);
        return id;
      } finally {
        setBusy(false);
      }
    },
    [mutate, publish, t, projectId],
  );

  return { create, busy };
}
