import { SetStartCommandDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseStartCommandResult {
  setStartCommand: (id: string, command: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Build & Deploy Start/Docker Command editor to bex-api. The toasts
 * name the field the card shows, so a Docker Command caller passes its own
 * keys (w4/357).
 */
export function useStartCommand(
  toasts: { success: string; error: string } = {
    success: "services.startCommandSuccess",
    error: "services.startCommandError",
  },
): UseStartCommandResult {
  const { run, busy } = useFieldMutation(
    SetStartCommandDocument,
    (id: string, command: string) => ({ id, command }),
    toasts,
  );

  return { setStartCommand: run, busy };
}
