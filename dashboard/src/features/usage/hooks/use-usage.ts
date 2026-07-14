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
  resourceKind: string;
  rows: UsageRow[];
}

export interface MeterEstimate {
  kind: string;
  tier: string;
  resourceKind: string;
  costUsd: string;
}

export interface EstimatedCost {
  totalUsd: string;
  meters: MeterEstimate[];
}

export interface UsageSummary {
  workspaceId: string;
  period: string; // "YYYY-MM"
  services: ServiceUsage[];
  estimatedCost: EstimatedCost | null;
}

export interface UseUsageResult {
  summary: UsageSummary | null;
  loading: boolean;
  error: Error | undefined;
}

export function useUsage(period?: string): UseUsageResult {
  const { data, loading, error } = useQuery(UsageDocument, {
    // {} and { period } key into separate Apollo cache entries — intentional so
    // current-month and historical queries never share a cache slot.
    variables: period ? { period } : {},
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
            period: raw.period ?? "",
            services: (raw.services ?? []).filter(Boolean).map((s) => ({
              serviceId: s!.serviceId ?? "",
              resourceKind: s!.resourceKind ?? "service",
              rows: (s!.rows ?? []).filter(Boolean).map((r) => ({
                kind: r!.kind ?? "",
                tier: r!.tier ?? "",
                total: r!.total ?? 0,
              })),
            })),
            estimatedCost: raw.estimatedCost
              ? {
                  totalUsd: raw.estimatedCost.totalUsd ?? "0.00",
                  meters: (raw.estimatedCost.meters ?? [])
                    .filter(Boolean)
                    .map((m) => ({
                      kind: m!.kind ?? "",
                      tier: m!.tier ?? "",
                      resourceKind: m!.resourceKind ?? "",
                      costUsd: m!.costUsd ?? "0.00",
                    })),
                }
              : null,
          }
        : null,
    [raw],
  );

  return { summary, loading, error };
}
