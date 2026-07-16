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
import { WorkspaceLimitsDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface ResourceCap {
  used: number;
  limit: number;
}

export interface ResourceLimits {
  services: ResourceCap;
  postgres: ResourceCap;
  keyValues: ResourceCap;
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
  });

  const raw = data?.workspaceLimits;
  const limits = raw
    ? {
        services: {
          used: raw.services?.used ?? 0,
          limit: raw.services?.limit ?? 0,
        },
        postgres: {
          used: raw.postgres?.used ?? 0,
          limit: raw.postgres?.limit ?? 0,
        },
        keyValues: {
          used: raw.keyValues?.used ?? 0,
          limit: raw.keyValues?.limit ?? 0,
        },
      }
    : null;

  return { limits, loading: !resolved || loading, error };
}
