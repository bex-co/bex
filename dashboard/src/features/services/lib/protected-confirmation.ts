import { graphQLErrorExtensions } from "@/common/lib/graphql-error";
import type { AskForConfirmation } from "@/common/providers/protected-retry-context";

export type ProtectedActionResult =
  | { status: "success" }
  | { status: "confirmation_required"; confirmation: string }
  | { status: "error" };

const PROTECTED_REFUSAL = "PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED";

/**
 * bex-api's protected-environment refusal: the authoritative retry phrase, and
 * the resource it names (the refusal's `name` param, w5/m130), or null for any
 * other error. The server computes the phrase from the actual verb and
 * immutable name; the dashboard deliberately does not duplicate that rule. The
 * name is display only, and falls back to the phrase.
 */
export function protectedRefusalFromError(
  err: unknown,
): { confirm: string; name: string } | null {
  const extensions = graphQLErrorExtensions(err, PROTECTED_REFUSAL);
  const confirm = extensions?.["confirm"];
  if (typeof confirm !== "string" || confirm === "") return null;
  const name = extensions?.["name"];
  return {
    confirm,
    name: typeof name === "string" && name !== "" ? name : confirm,
  };
}

/** The refusal's retry phrase, for a caller that names the resource itself. */
export function protectedConfirmationFromError(err: unknown): string | null {
  return protectedRefusalFromError(err)?.confirm ?? null;
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
  ask: AskForConfirmation,
  attempt: (confirm?: string) => Promise<T>,
): Promise<T> {
  try {
    return await attempt();
  } catch (err) {
    const refusal = protectedRefusalFromError(err);
    if (!refusal) throw err;
    const confirmation = await ask(refusal.confirm, refusal.name);
    if (confirmation === null) throw new ProtectedConfirmationDismissed();
    return attempt(confirmation);
  }
}
