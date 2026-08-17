import { useMutation } from "@apollo/client/react";
import type { EnvironmentPatchInput } from "@/features/services/lib/environment-draft";
import { PatchServiceEnvironmentDocument } from "@/graphql/definitions";

export type EnvironmentSaveMode = "save_only" | "deploy";

export function useEnvironmentDraftSave() {
  // Refetch only the queries that display the patch's result — the env-var and
  // secret-file lists the editor's read view renders, and Server (header/env
  // state) — not every active query. Awaiting keeps the saving state up until
  // the read view's data is fresh, so ending the draft never flashes pre-save
  // values.
  const [mutate, { loading }] = useMutation(PatchServiceEnvironmentDocument, {
    refetchQueries: ["Server", "EnvVarKeys", "SecretFileNames"],
    awaitRefetchQueries: true,
  });

  async function save(
    serviceId: string,
    patch: EnvironmentPatchInput,
    saveMode: EnvironmentSaveMode,
  ) {
    const { data } = await mutate({
      variables: { serviceId, ...patch, saveMode },
    });
    if (!data?.patchServiceEnvironment) {
      throw new Error("environment patch returned no result");
    }
    return data.patchServiceEnvironment;
  }

  return { save, saving: loading };
}
