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

import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  DatabaseProcessesDocument,
  DatabaseTopQueriesDocument,
  DatabaseSizesDocument,
  DatabaseTableScansDocument,
  DatabaseParameterOverridesDocument,
  DatabaseParameterSpecDocument,
} from "@/graphql/definitions";
import { nonNull } from "@/common/lib/non-null";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";

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
    pollInterval: 15_000,
    skipPollAttempt: skipPollWhenHidden,
  });

  // The DECLARED overrides (Database.spec.parameters) — read-only since
  // parameterOverrides was retired (w2/066); a database that declared some
  // before then keeps applying them. parameterOverrides above is the observed
  // pg_settings config, most of which the operator owns (w6/m133).
  const parameterSpec = useQuery(DatabaseParameterSpecDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: 15_000,
    skipPollAttempt: skipPollWhenHidden,
  });

  // The generated result types carry nullable list items; the panel renders
  // concrete rows, so drop the nulls here once. Memoized per query so a poll
  // tick that returns identical data keeps identical references (Apollo hands
  // back the same cache objects; a bare `.filter` would re-allocate every
  // render on this polling page).
  const processesData = processes.data;
  const processRows = useMemo(
    () => (processesData?.databaseProcesses ?? []).filter(nonNull),
    [processesData],
  );
  const topQueriesData = topQueries.data;
  const topQueryRows = useMemo(
    () => (topQueriesData?.databaseTopQueries ?? []).filter(nonNull),
    [topQueriesData],
  );
  const sizesData = sizes.data;
  const sizesView = useMemo(() => {
    const raw = sizesData?.databaseSizes;
    return raw
      ? { database: raw.database, tables: (raw.tables ?? []).filter(nonNull) }
      : null;
  }, [sizesData]);
  const tableScansData = tableScans.data;
  const tableScanRows = useMemo(
    () => (tableScansData?.databaseTableScans ?? []).filter(nonNull),
    [tableScansData],
  );
  const parameterSpecData = parameterSpec.data;
  const parameterSpecRows = useMemo(
    () => (parameterSpecData?.databaseParameterSpec ?? []).filter(nonNull),
    [parameterSpecData],
  );
  const parameterOverridesData = parameterOverrides.data;
  const parameterOverrideRows = useMemo(
    () =>
      (parameterOverridesData?.databaseParameterOverrides ?? []).filter(
        nonNull,
      ),
    [parameterOverridesData],
  );

  return {
    processes: processRows,
    processesLoading: processes.loading,
    processesError: processes.error,

    topQueries: topQueryRows,
    topQueriesLoading: topQueries.loading,
    topQueriesError: topQueries.error,

    sizes: sizesView,
    sizesLoading: sizes.loading,
    sizesError: sizes.error,

    tableScans: tableScanRows,
    tableScansLoading: tableScans.loading,
    tableScansError: tableScans.error,

    parameterOverrides: parameterOverrideRows,
    parameterOverridesLoading: parameterOverrides.loading,
    parameterOverridesError: parameterOverrides.error,

    parameterSpec: parameterSpecRows,

    refetchAll() {
      void processes.refetch();
      void topQueries.refetch();
      void sizes.refetch();
      void tableScans.refetch();
      void parameterOverrides.refetch();
      void parameterSpec.refetch();
    },
  };
}
