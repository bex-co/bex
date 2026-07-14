import { useMutation } from "@apollo/client/react";
import {
  ExecuteDatabaseQueryDocument,
  type DatabaseQueryResult,
} from "@/features/databases/api/operations";

export interface QueryResult {
  columns: string[];
  rows: Array<Array<string | null>>;
  rowCount: number;
  truncated: boolean;
}

/** Executes one console statement through bex-api's scoped GraphQL mutation. */
export function useExecuteDatabaseQuery(id: string) {
  const [mutate, { loading }] = useMutation(ExecuteDatabaseQueryDocument);

  async function execute(
    sql: string,
    allowWrites: boolean,
  ): Promise<QueryResult> {
    const response = await mutate({ variables: { id, sql, allowWrites } });
    const result = response.data?.executeDatabaseQuery;
    if (!result) throw new Error();
    return normalizeResult(result);
  }

  return { execute, loading };
}

function normalizeResult(result: DatabaseQueryResult): QueryResult {
  return {
    columns: (result.columns ?? []).map((column) => column ?? ""),
    rows: (result.rows ?? []).map((row) => [...(row?.values ?? [])]),
    rowCount: result.rowCount ?? 0,
    truncated: result.truncated ?? false,
  };
}
