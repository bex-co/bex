import { useMutation } from "@apollo/client/react";
import type { EnvironmentPatchInput } from "@/features/services/lib/environment-draft";
import { PatchServiceEnvironmentDocument } from "@/graphql/definitions";

export type EnvironmentSaveMode = "save_only" | "deploy";

export function useEnvironmentDraftSave() {
  const [mutate, { loading }] = useMutation(PatchServiceEnvironmentDocument, {
    refetchQueries: "active",
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
