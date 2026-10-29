import { SetDockerContextDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseDockerContextResult {
  setDockerContext: (id: string, dockerContext: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Docker Build Context Directory editor to bex-api's existing-service
 * setter (w8/m128). The value is relative to the Root Directory, as on Render;
 * an empty value restores the default, the Root Directory itself.
 */
export function useDockerContext(): UseDockerContextResult {
  const { run, busy } = useFieldMutation(
    SetDockerContextDocument,
    (id: string, dockerContext: string) => ({ id, dockerContext }),
    {
      success: "services.dockerContextSuccess",
      error: "services.dockerContextError",
    },
  );

  return { setDockerContext: run, busy };
}
