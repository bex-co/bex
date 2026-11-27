import { useCallback } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { UpdateEnvironmentDocument } from "@/graphql/definitions";
import type { EnvironmentIPAllowListEntry } from "@/features/environments/hooks/use-environments";
import { refreshEnvironments } from "@/features/environments/lib/refresh-environments";

/** The fields one settings card saves; anything left out stays as it is. */
export interface EnvironmentPatch {
  name?: string;
  protectedStatus?: "protected" | "unprotected";
  networkIsolationEnabled?: boolean;
  ipAllowListEntries?: EnvironmentIPAllowListEntry[];
}

export type EnvironmentUpdateResult =
  | { ok: true }
  | { ok: false; error: unknown };

/**
 * Render's `updateEnvironment` patch: each card of the environment settings
 * page, and the section menu's isolation toggle, saves only its own field. The
 * answer is the environment, which the cache writes into every list holding
 * it at once; the project's list is then re-read behind the save, after any
 * read already in flight, which would otherwise land later with the old value
 * (`refreshEnvironments`). The refusal comes back to the caller, which shows it
 * where Render does (under the field, on the card, in the dialog).
 */
export function useUpdateEnvironment() {
  const client = useApolloClient();
  const [mutate, { loading }] = useMutation(UpdateEnvironmentDocument);

  const update = useCallback(
    async (
      id: string,
      patch: EnvironmentPatch,
    ): Promise<EnvironmentUpdateResult> => {
      let projectId: string | null | undefined;
      try {
        const result = await mutate({ variables: { id, ...patch } });
        projectId = result.data?.updateEnvironment?.projectId;
      } catch (error) {
        return { ok: false, error };
      }
      if (projectId) {
        refreshEnvironments(client, projectId).catch(() => undefined);
      }
      return { ok: true };
    },
    [client, mutate],
  );

  return { update, saving: loading };
}
