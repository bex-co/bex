import { createContext, useContext } from "react";
import type { UseDatabaseLifecycleResult } from "@/features/databases/hooks/use-database-lifecycle";
import type { DatabaseDetailView } from "@/features/databases/types";

/**
 * What a Postgres database's layout route hands each of its pages (w3/m641):
 * the database it read, the re-read that converges its status after a change,
 * and the lifecycle verbs the header and the Info page's danger zone share.
 * The layout mounts its `<Outlet/>` only once the database is loaded and its
 * workspace is the selection, so a page never renders without one.
 */
export interface DatabaseDetail {
  database: DatabaseDetailView;
  refetch: () => void;
  lifecycle: UseDatabaseLifecycleResult;
}

const DatabaseDetailContext = createContext<DatabaseDetail | null>(null);

export const DatabaseDetailProvider = DatabaseDetailContext.Provider;

/** The open database, for a page under `/databases/$databaseId`. */
export function useDatabaseDetail(): DatabaseDetail {
  const detail = useContext(DatabaseDetailContext);
  if (!detail) {
    throw new Error("useDatabaseDetail outside the database layout");
  }
  return detail;
}

/**
 * The open database when there is one: a page's pending frame also paints in
 * the layout's own loading frame, before any database is known.
 */
export function useOptionalDatabaseDetail(): DatabaseDetail | null {
  return useContext(DatabaseDetailContext);
}
