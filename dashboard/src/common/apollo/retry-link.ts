import type { ApolloLink, ErrorLike } from "@apollo/client";
import { ServerError } from "@apollo/client/errors";
import { RetryLink } from "@apollo/client/link/retry";
import { getMainDefinition } from "@apollo/client/utilities";

/**
 * Whether a failed operation is safe and worth retrying (w1/m52 t003): READS
 * only — mutations are not idempotent in general, so a transport blip on a
 * write stays a surfaced error the user retries explicitly — and only for
 * failures that look transient:
 *
 * - a network/transport failure (fetch rejected — the link error has no HTTP
 *   status), e.g. the connection reset when a pod rolls, and
 * - 5xx/429 responses, e.g. the LB's 502 while endpoints catch up.
 *
 * 4xx like 401/403/400 are deterministic answers, not blips — retrying them
 * only delays the real error state. GraphQL execution errors never reach a
 * RetryLink at all (they arrive as a result, not a link error), which is
 * exactly the contract the tests pin.
 */
export function shouldRetry(
  error: ErrorLike,
  operation: ApolloLink.Operation,
): boolean {
  const definition = getMainDefinition(operation.query);
  const isQuery =
    definition.kind === "OperationDefinition" &&
    definition.operation === "query";
  if (!isQuery) return false;
  if (ServerError.is(error)) {
    return error.statusCode >= 500 || error.statusCode === 429;
  }
  return true;
}

/**
 * The retry link both Apollo clients (SSR and browser) put in front of their
 * HTTP link: up to 2 retries with short jittered backoff, so a roll-window
 * transport failure self-heals instead of dehydrating a stranded error state.
 */
export function createRetryLink() {
  return new RetryLink({
    attempts: { max: 3, retryIf: shouldRetry },
    delay: { initial: 300, max: 2000, jitter: true },
  });
}
