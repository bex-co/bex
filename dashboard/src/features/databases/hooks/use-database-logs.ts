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

import { useQuery } from "@apollo/client/react";
import {
  DatabaseLogsDocument,
  type DatabaseLogsVars,
} from "@/features/databases/api/operations";

/**
 * Fetches live Postgres pod logs for a managed database.
 * CNPG pods are NOT shipped to Loki, so this is a direct pod-log read —
 * no durable history; results reflect only currently running pods.
 */
export function useDatabaseLogs(vars: DatabaseLogsVars) {
  const { data, loading, error, refetch } = useQuery(DatabaseLogsDocument, {
    variables: vars,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  return {
    entries: data?.databaseLogs ?? [],
    loading,
    error,
    refetch: () => void refetch(),
  };
}
