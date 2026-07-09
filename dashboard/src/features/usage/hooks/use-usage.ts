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
import { useMemo } from "react";
import { UsageDocument } from "@/graphql/definitions";

export interface UsageRow {
  kind: string;
  tier: string;
  total: number;
}

export interface ServiceUsage {
  serviceId: string;
  rows: UsageRow[];
}

export interface UsageSummary {
  workspaceId: string;
  services: ServiceUsage[];
}

export interface UseUsageResult {
  summary: UsageSummary | null;
  loading: boolean;
  error: Error | undefined;
}

export function useUsage(): UseUsageResult {
  const { data, loading, error } = useQuery(UsageDocument, {
    pollInterval: 60_000,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const raw = data?.usage;
  const summary = useMemo<UsageSummary | null>(
    () =>
      raw
        ? {
            workspaceId: raw.workspaceId ?? "",
            services: (raw.services ?? [])
              .filter(Boolean)
              .map((s) => ({
                serviceId: s!.serviceId ?? "",
                rows: (s!.rows ?? [])
                  .filter(Boolean)
                  .map((r) => ({
                    kind: r!.kind ?? "",
                    tier: r!.tier ?? "",
                    total: r!.total ?? 0,
                  })),
              })),
          }
        : null,
    [raw],
  );

  return { summary, loading, error };
}
