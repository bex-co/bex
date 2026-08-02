import { NetworkStatus } from "@apollo/client";

/**
 * Writes may be exposed only after the current network request completed with
 * coherent data. Cache-and-network snapshots and failed refreshes remain useful
 * for read-only display, but never authorize a mobile control.
 */
export function hasAuthoritativeCurrentEvidence(input: {
  networkStatus: NetworkStatus;
  error: unknown;
  hasData: boolean;
}): boolean {
  return (
    input.hasData &&
    input.error == null &&
    input.networkStatus === NetworkStatus.ready
  );
}
