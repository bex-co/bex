import { graphQLErrorMessage } from "@/common/lib/graphql-error";

export type ProtectedActionResult =
  | { status: "success" }
  | { status: "confirmation_required"; confirmation: string }
  | { status: "error" };

/**
 * Pulls the authoritative retry phrase out of bex-api's protected-environment
 * error. The server computes the phrase from the actual verb and immutable
 * service name; the dashboard deliberately does not duplicate that rule.
 */
export function protectedConfirmationFromError(err: unknown): string | null {
  const message = graphQLErrorMessage(err);
  if (!message) return null;
  // Two server handshakes share the confirm-phrase convention: the
  // protected-environment guard and the blueprint ownership takeover (w8/m23).
  if (
    !message.includes("protected environment") &&
    !message.includes("is managed by blueprint")
  ) {
    return null;
  }
  const match = message.match(/retry with confirm=(?:"([^"]+)"|'([^']+)')/i);
  return match?.[1] ?? match?.[2] ?? null;
}

/** Best-effort display name; authorization still relies on the full phrase. */
export function protectedServiceName(confirmation: string): string {
  // The verb is not always one word — "fail over" a database, "take offline" a
  // service — so match lazily up to the resource kind rather than assuming it.
  return (
    confirmation.match(/^sudo .+? (?:service|database|key value) (.+)$/)?.[1] ??
    confirmation
  );
}

/**
 * Thrown when the user dismisses the protected-environment retry dialog. It is
 * not a failure to report: the save simply did not happen, and the user is the
 * one who decided that, so a caller catches it and returns quietly rather than
 * toasting an error at someone who just pressed Cancel.
 */
export class ProtectedConfirmationDismissed extends Error {
  constructor() {
    super("protected confirmation dismissed");
    this.name = "ProtectedConfirmationDismissed";
  }
}

/**
 * Runs a mutation, and if bex-api refuses it because the resource belongs to a
 * protected environment, asks for the phrase the server issued and runs it once
 * more with that phrase (w4/m126).
 *
 * `attempt` receives the confirmation to pass through as the mutation's
 * `confirm` argument — undefined on the first, unconfirmed try. Any other error
 * propagates untouched, so each call site keeps its own error copy.
 */
export async function withProtectedRetry<T>(
  ask: (phrase: string) => Promise<string | null>,
  attempt: (confirm?: string) => Promise<T>,
): Promise<T> {
  try {
    return await attempt();
  } catch (err) {
    const phrase = protectedConfirmationFromError(err);
    if (!phrase) throw err;
    const confirmation = await ask(phrase);
    if (confirmation === null) throw new ProtectedConfirmationDismissed();
    return attempt(confirmation);
  }
}
