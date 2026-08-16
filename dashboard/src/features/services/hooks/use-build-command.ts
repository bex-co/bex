import { SetBuildCommandDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseBuildCommandResult {
  setBuildCommand: (id: string, command: string) => Promise<boolean>;
  busy: boolean;
}

/** Wires the Build & Deploy Build Command editor to bex-api. */
export function useBuildCommand(): UseBuildCommandResult {
  const { run, busy } = useFieldMutation(
    SetBuildCommandDocument,
    (id: string, command: string) => ({ id, command }),
    {
      success: "services.buildCommandSuccess",
      error: "services.buildCommandError",
    },
  );

  return { setBuildCommand: run, busy };
}
