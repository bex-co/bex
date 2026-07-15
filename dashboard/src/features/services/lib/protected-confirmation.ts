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
  if (!message || !message.includes("protected environment")) return null;
  const match = message.match(/retry with confirm=(?:"([^"]+)"|'([^']+)')/i);
  return match?.[1] ?? match?.[2] ?? null;
}

/** Best-effort display name; authorization still relies on the full phrase. */
export function protectedServiceName(confirmation: string): string {
  return confirmation.match(/^sudo \S+ service (.+)$/)?.[1] ?? confirmation;
}
