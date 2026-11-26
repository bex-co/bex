import { useQuery } from "@apollo/client/react";
import { DatabaseStorageDocument } from "@/graphql/definitions";
import type { DatabaseStorageUsage } from "@/features/databases/lib/storage";

/**
 * Reads the database's storage usage for Render's almost-full warning
 * (w3/m656), apart from the page's own read so the page never waits on it.
 * Each page that shows the warning reads it afresh when it opens; null until
 * it answers, and when it fails.
 */
export function useDatabaseStorage(id: string): DatabaseStorageUsage | null {
  const { data } = useQuery(DatabaseStorageDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  return data?.database ?? null;
}
