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
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UsageRow {
  kind: string;
  tier: string;
  total: number;
}

export interface ServiceUsage {
  serviceId: string;
  /** User-facing display name; empty when the resource no longer exists — fall back to serviceId. */
  serviceName: string;
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

/** A normalized Metronome amount over a period (m48). */
export interface BillingAmount {
  amountUsd: string;
  currency: string;
  periodStart: string; // RFC3339
  periodEnd: string; // RFC3339
}

/** One finalized Metronome invoice (m48). */
export interface BillingInvoice {
  id: string;
  status: string;
  amountUsd: string;
  currency: string;
  periodStart: string;
  periodEnd: string;
}

/**
 * Real Metronome-computed billing (m48). Distinct from the advisory
 * estimatedCost: this is what the workspace is actually charged. null when
 * estimate-only (no contract, comped/excluded, billing off, or degraded).
 */
export interface Billing {
  currentCost: BillingAmount | null;
  invoices: BillingInvoice[];
}

export interface UsageSummary {
  workspaceId: string;
  period: string; // "YYYY-MM"
  services: ServiceUsage[];
  estimatedCost: EstimatedCost | null;
  billing: Billing | null;
}

export interface UseUsageResult {
  summary: UsageSummary | null;
  loading: boolean;
  error: Error | undefined;
}

export function useUsage(period?: string): UseUsageResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error } = useQuery(UsageDocument, {
    // { ownerId } and { period, ownerId } key into separate Apollo cache
    // entries — intentional so current-month and historical queries never
    // share a cache slot. Skipped until the switcher's selection resolves, so
    // this never fires with a null ownerId (which the backend would silently
    // route to the caller's default workspace, w6/m18) then refetch once it does.
    variables: period
      ? { period, ownerId: currentWorkspaceId }
      : { ownerId: currentWorkspaceId },
    skip: !resolved,
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
              serviceName: s!.serviceName ?? "",
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
            billing: raw.billing
              ? {
                  currentCost: raw.billing.currentCost
                    ? {
                        amountUsd: raw.billing.currentCost.amountUsd ?? "0.00",
                        currency: raw.billing.currentCost.currency ?? "USD",
                        periodStart: raw.billing.currentCost.periodStart ?? "",
                        periodEnd: raw.billing.currentCost.periodEnd ?? "",
                      }
                    : null,
                  invoices: (raw.billing.invoices ?? [])
                    .filter(Boolean)
                    .map((i) => ({
                      id: i!.id ?? "",
                      status: i!.status ?? "",
                      amountUsd: i!.amountUsd ?? "0.00",
                      currency: i!.currency ?? "USD",
                      periodStart: i!.periodStart ?? "",
                      periodEnd: i!.periodEnd ?? "",
                    })),
                }
              : null,
          }
        : null,
    [raw],
  );

  return { summary, loading: !resolved || loading, error };
}
