import { SetStartCommandDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseStartCommandResult {
  setStartCommand: (id: string, command: string) => Promise<boolean>;
  busy: boolean;
}

/** Wires the Build & Deploy Start/Docker Command editor to bex-api. */
export function useStartCommand(): UseStartCommandResult {
  const { run, busy } = useFieldMutation(
    SetStartCommandDocument,
    (id: string, command: string) => ({ id, command }),
    {
      success: "services.startCommandSuccess",
      error: "services.startCommandError",
    },
  );

  return { setStartCommand: run, busy };
}
