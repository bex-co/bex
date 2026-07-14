import { useQuery } from "@apollo/client/react";
import { ServiceNameAvailableDocument } from "@/graphql/definitions";
import { useDebounce } from "@/common/hooks/use-debounce";
import { isValidDnsLabel } from "@/common/lib/utils/dns-label";

export interface UseServiceNameAvailabilityResult {
  /** True once a debounced check for `name` has come back taken. */
  taken: boolean;
  /** The next free "<name>-N" name, set only when `taken`. */
  suggestion: string | null;
  /** A check is in flight for the (debounced) current name. */
  checking: boolean;
}

/**
 * Debounced wrapper around `serviceNameAvailable` (w4/m19) — the create
 * form's live "Name is already in use" check. Skips the query entirely for
 * an empty or already-invalid name (nothing to ask the backend), so typing
 * never fires a request for a name that would fail client-side validation
 * anyway.
 */
export function useServiceNameAvailability(
  name: string,
): UseServiceNameAvailabilityResult {
  const debounced = useDebounce(name, 400);
  const queryable = debounced.length > 0 && isValidDnsLabel(debounced);

  const { data, loading } = useQuery(ServiceNameAvailableDocument, {
    variables: { name: debounced },
    skip: !queryable,
    fetchPolicy: "cache-and-network",
  });

  const result = data?.serviceNameAvailable;
  // Only report "taken" once the debounced name is what the check ran
  // against — otherwise a stale prior result would flash between keystrokes
  // while the newer request is still in flight.
  const settledForCurrentName = queryable && debounced === name;

  return {
    taken: settledForCurrentName && result?.available === false,
    suggestion: settledForCurrentName ? (result?.suggestion ?? null) : null,
    checking: queryable && loading,
  };
}
