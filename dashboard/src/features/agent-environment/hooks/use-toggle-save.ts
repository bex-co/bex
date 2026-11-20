import { useState } from "react";
import { toast } from "sonner";
import type { AgentEnvironmentInput } from "@/graphql/definitions";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";

/**
 * Save-on-toggle for a Settings → Agents switch section (the sessions' gh,
 * w10/m524; the models members can choose, w10/m527). `pending` is the value
 * being saved, which the switches show until the answer reaches `saved`
 * through the Apollo cache; a refused save drops it, so they show the stored
 * value again, and keeps the refusal's locale key for `ToggleSaveAlert`.
 */
export function useToggleSave<T>(
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>,
  errorKeyOf: (err: unknown) => string,
) {
  const [pending, setPending] = useState<T | null>(null);
  const [errorKey, setErrorKey] = useState<string | null>(null);

  /** Saves `input` while showing `value`, then toasts `savedMessage(answer)`. */
  async function save(
    value: T,
    input: AgentEnvironmentInput,
    savedMessage: (next: AgentEnvironmentView) => string,
  ) {
    setPending(value);
    setErrorKey(null);
    try {
      toast.success(savedMessage(await update(input)));
    } catch (err) {
      setErrorKey(errorKeyOf(err));
    } finally {
      setPending(null);
    }
  }

  return { pending, errorKey, save };
}
