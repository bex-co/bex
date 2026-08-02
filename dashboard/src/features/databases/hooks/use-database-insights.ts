/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useQuery, useMutation } from "@apollo/client/react";
import {
  DatabaseProcessesDocument,
  DatabaseTopQueriesDocument,
  DatabaseSizesDocument,
  DatabaseTableScansDocument,
  DatabaseParameterOverridesDocument,
  SetDatabaseParameterOverridesDocument,
  type ParameterInput,
} from "@/features/databases/api/operations";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";

export type SaveParametersResult =
  { ok: true } | { ok: false; error: string | null };

/**
 * Returns all five insight datasets for a managed-Postgres database.
 * All five queries run concurrently on mount; each has its own loading/error
 * state so a partial failure (e.g. pg_stat_statements not installed → empty
 * top-queries) doesn't block the others from rendering.
 */
export function useDatabaseInsights(id: string) {
  const processes = useQuery(DatabaseProcessesDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const topQueries = useQuery(DatabaseTopQueriesDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const sizes = useQuery(DatabaseSizesDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const tableScans = useQuery(DatabaseTableScansDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const parameterOverrides = useQuery(DatabaseParameterOverridesDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    // The operator applies postgresql.conf changes asynchronously. Keep the
    // live pg_settings view moving until the saved setting/source appears.
    pollInterval: 15_000,
  });

  const [setParameters, { loading: saving }] = useMutation(
    SetDatabaseParameterOverridesDocument,
  );

  async function saveParameters(
    params: ParameterInput[],
  ): Promise<SaveParametersResult> {
    try {
      await setParameters({ variables: { id, parameters: params } });
      void parameterOverrides.refetch();
      return { ok: true };
    } catch (err) {
      return { ok: false, error: graphQLErrorMessage(err) };
    }
  }

  return {
    processes: processes.data?.databaseProcesses ?? [],
    processesLoading: processes.loading,
    processesError: processes.error,

    topQueries: topQueries.data?.databaseTopQueries ?? [],
    topQueriesLoading: topQueries.loading,
    topQueriesError: topQueries.error,

    sizes: sizes.data?.databaseSizes ?? null,
    sizesLoading: sizes.loading,
    sizesError: sizes.error,

    tableScans: tableScans.data?.databaseTableScans ?? [],
    tableScansLoading: tableScans.loading,
    tableScansError: tableScans.error,

    parameterOverrides:
      parameterOverrides.data?.databaseParameterOverrides ?? [],
    parameterOverridesLoading: parameterOverrides.loading,
    parameterOverridesError: parameterOverrides.error,

    saving,
    saveParameters,

    refetchAll() {
      void processes.refetch();
      void topQueries.refetch();
      void sizes.refetch();
      void tableScans.refetch();
      void parameterOverrides.refetch();
    },
  };
}
