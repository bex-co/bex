import type { ApolloClient } from "@apollo/client";
import { KeyValueDocument } from "@/graphql/definitions";

/**
 * Let a store's detail reads that were already in flight when a configuration
 * save landed finish first (w4/m226). The polled KeyValue read carries the
 * persistence mode and eviction policy, so one sent before the write would
 * otherwise put the old value back after it; the caller then re-asserts the
 * saved value, which is authoritative as of the write.
 */
export async function settleKeyValueDetailReads(
  client: ApolloClient,
  id: string,
): Promise<void> {
  const reads = client.refetchQueries({
    include: [KeyValueDocument],
    onQueryUpdated: (query) =>
      (query.variables as { id?: string }).id === id &&
      query.getCurrentResult().loading,
  });
  reads.catch(() => undefined); // each result is settled below
  await Promise.allSettled(reads.results);
}
