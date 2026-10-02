import { useMutation } from "@apollo/client/react";
import type { EnvironmentPatchInput } from "@/features/services/lib/environment-draft";
import { PatchServiceEnvironmentDocument } from "@/graphql/definitions";

export type EnvironmentSaveMode = "save_only" | "deploy";

export function useEnvironmentDraftSave() {
  // Refetch only the queries that display the patch's result — the env-var and
  // secret-file lists the editor's read view renders, and Server (header/env
  // state) — not every active query. Awaiting keeps the saving state up until
  // the read view's data is fresh, so ending the draft never flashes pre-save
  // values. Server status is reconciled asynchronously: a fresh response can
  // still precede the operator's comparison. Leave that flag server-owned
  // (a no-op or revert may have no difference); the save toast explains that
  // this action did not deploy while the layout's poll picks up the result.
  const [mutate, { loading }] = useMutation(PatchServiceEnvironmentDocument, {
    refetchQueries: ["Server", "EnvVarKeys", "SecretFileNames"],
    awaitRefetchQueries: true,
  });

  async function save(
    serviceId: string,
    patch: EnvironmentPatchInput,
    saveMode: EnvironmentSaveMode,
  ) {
    let refreshFailed = false;
    const { data } = await mutate({
      variables: { serviceId, ...patch, saveMode },
      onQueryUpdated: async (query) => {
        // The mutation has already committed. A failed follow-up read must
        // never keep the draft open as though replaying the patch were safe.
        try {
          const result = await query.refetch().retain();
          refreshFailed ||= Boolean(result.error);
        } catch {
          refreshFailed = true;
        }
      },
    });
    if (!data?.patchServiceEnvironment) {
      throw new Error("environment patch returned no result");
    }
    return { ...data.patchServiceEnvironment, refreshFailed };
  }

  return { save, saving: loading };
}
