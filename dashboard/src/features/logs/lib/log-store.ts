import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";

/**
 * True when bex-api refused a read only the durable log store can answer
 * (request logs, build-log history, structured filters, a host/path-filtered
 * request metric) because the deployment has none
 * (core.ErrLogStoreUnavailable, 503). Callers render an explanatory state for
 * it rather than a generic error.
 */
export function isLogStoreUnavailable(error: unknown): boolean {
  return hasGraphQLErrorCode(error, "LOG_STORE_UNAVAILABLE");
}
