// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { useQuery } from "@apollo/client/react";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { WorkspaceLimitsDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface ResourceCap {
  used: number;
  // How many of `used` are finishing deletion. Those rows are dropped from the
  // resource list but still hold quota (w6/m129), so surfacing this is what lets
  // the usage figure reconcile with the shorter list: used - terminating is the
  // count the list shows.
  terminating: number;
  limit: number;
}

export interface ResourceLimits {
  services: ResourceCap;
  postgres: ResourceCap;
  keyValues: ResourceCap;
}

// Coalesce one GraphQL cap (every field nullable) into a defined ResourceCap, so
// the reconciliation arithmetic used - terminating always has real numbers. A
// missing terminating defaults to 0, never undefined.
function toCap(
  cap:
    | {
        used?: number | null;
        terminating?: number | null;
        limit?: number | null;
      }
    | null
    | undefined,
): ResourceCap {
  return {
    used: cap?.used ?? 0,
    terminating: cap?.terminating ?? 0,
    limit: cap?.limit ?? 0,
  };
}

export function useResourceLimits(): {
  limits: ResourceLimits | null;
  loading: boolean;
  error: Error | undefined;
} {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error } = useQuery(WorkspaceLimitsDocument, {
    variables: { ownerId: currentWorkspaceId ?? "" },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    // Counts change through three independent create/delete feature areas.
    // Polling keeps this cross-feature summary correct without coupling every
    // mutation hook to the Usage page's query.
    pollInterval: 15_000,
    skipPollAttempt: skipPollWhenHidden,
  });

  const raw = data?.workspaceLimits;
  const limits = raw
    ? {
        services: toCap(raw.services),
        postgres: toCap(raw.postgres),
        keyValues: toCap(raw.keyValues),
      }
    : null;

  return { limits, loading: !resolved || loading, error };
}
