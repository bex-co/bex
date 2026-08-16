import { SetDockerfilePathDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseDockerfilePathResult {
  setDockerfilePath: (id: string, dockerfilePath: string) => Promise<boolean>;
  busy: boolean;
}

/** Wires the Dockerfile Path editor to bex-api's existing-service setter. */
export function useDockerfilePath(): UseDockerfilePathResult {
  const { run, busy } = useFieldMutation(
    SetDockerfilePathDocument,
    (id: string, dockerfilePath: string) => ({ id, dockerfilePath }),
    {
      success: "services.dockerfilePathSuccess",
      error: "services.dockerfilePathError",
    },
  );

  return { setDockerfilePath: run, busy };
}
