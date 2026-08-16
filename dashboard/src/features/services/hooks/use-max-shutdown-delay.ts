import { SetMaxShutdownDelayDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseMaxShutdownDelayResult {
  setMaxShutdownDelay: (id: string, seconds: number) => Promise<boolean>;
  busy: boolean;
}

/** Wires Settings to the shared maxShutdownDelaySeconds service verb. */
export function useMaxShutdownDelay(): UseMaxShutdownDelayResult {
  const { run, busy } = useFieldMutation(
    SetMaxShutdownDelayDocument,
    (id: string, seconds: number) => ({ id, seconds }),
    {
      success: "services.maxShutdownDelaySuccess",
      error: "services.maxShutdownDelayError",
    },
  );

  return { setMaxShutdownDelay: run, busy };
}
