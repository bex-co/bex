import { createContext, useContext } from "react";

/**
 * The protected-environment handshake seam (w4/m126).
 *
 * Delete and suspend already did this per call site: catch the server's named
 * refusal, show the retry dialog with the phrase the server issued, retry with
 * `confirm`. w4/m126 extended the guard to every verb that repoints a service,
 * redefines how it is built or started, or takes it offline — which is most of
 * the Settings tab, and re-spelling the catch/dialog/retry dance in a dozen
 * single-field editors is how it would land in eleven of them.
 *
 * So the dialog lives in one mount above the routes (ProtectedRetryProvider)
 * and a hook asks it for a phrase, knowing nothing about rendering.
 *
 * The phrase is never constructed here — `askForConfirmation` is only ever
 * called with the string parsed out of the server's error, which is ADR032's
 * rule ("the dashboard does not precompute the phrase") and the reason a
 * verb-word change in bex-api cannot desynchronize the UI.
 */
export type AskForConfirmation = (phrase: string) => Promise<string | null>;

export const ProtectedRetryContext = createContext<AskForConfirmation | null>(
  null,
);

const noRetry: AskForConfirmation = () => Promise.resolve(null);

/**
 * Returns a function that shows the retry dialog for a server-issued phrase and
 * resolves with what the user typed, or null if they dismissed it. Outside the
 * provider it resolves null, so a component rendered in isolation (a test, a
 * storybook) degrades to "no retry offered" rather than throwing.
 */
export function useAskForProtectedConfirmation(): AskForConfirmation {
  const ask = useContext(ProtectedRetryContext);
  return ask ?? noRetry;
}
