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
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";
import { useMemo } from "react";
import { UsageDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * One meter's contribution to a resource's cost. Rate and quantity are both
 * quoted in `unit`, so `rateUsd × quantity = costUsd` reads correctly on the
 * page — the backend owns that conversion because it owns the price sheet.
 */
export interface ChargeLine {
  kind: string;
  tier: string;
  /** Display unit for both rate and quantity: "hr", "GB", "min", "GB-mo", "vCPU-hr". */
  unit: string;
  rateUsd: string;
  quantity: string;
  costUsd: string;
}

/** One resource's estimated cost for the period, with its charge lines. */
export interface ResourceEstimate {
  serviceId: string;
  /** User-facing display name; empty when the resource no longer exists — fall back to serviceId. */
  serviceName: string;
  resourceKind: string;
  costUsd: string;
  charges: ChargeLine[];
}

export interface EstimatedCost {
  totalUsd: string;
  resources: ResourceEstimate[];
}

/**
 * A normalized Stripe invoice amount over a period (m48/m50).
 *
 * Three figures, not one: under a credit grant Stripe's invoice total is
 * already net of the credit, so a single number could not say both what the
 * period cost and what is owed — a fully-credited period read "$0.00 month to
 * date" beside a nonzero charge tree (w6/m98).
 */
export interface BillingAmount {
  /** Gross rated charge, before discounts, credit, and tax. */
  amountUsd: string;
  /** Billing credit Stripe applied to this period; "0.00" without a grant. */
  creditsAppliedUsd: string;
  /** What Stripe actually collects, after discounts, credit, and tax. */
  amountDueUsd: string;
  currency: string;
  periodStart: string; // RFC3339
  periodEnd: string; // RFC3339
}

/** One finalized Stripe invoice (m48/m50); same three-figure split. */
export interface BillingInvoice {
  id: string;
  status: string;
  amountUsd: string;
  creditsAppliedUsd: string;
  amountDueUsd: string;
  currency: string;
  periodStart: string;
  periodEnd: string;
}

/** One active credit grant's remaining balance (w5/m70). */
export interface BillingCreditGrant {
  name: string;
  remainingUsd: string;
  /** RFC3339; empty for a grant that never expires. Earliest-expiring first. */
  expiresAt: string;
}

/**
 * Remaining Stripe billing-credit balance (w5/m70). null when the balance is
 * zero or the read degrades — the UI hides credit chrome entirely then.
 */
export interface BillingCredits {
  availableUsd: string;
  currency: string;
  grants: BillingCreditGrant[];
}

/**
 * Real Stripe billing (m48/m50). Distinct from the advisory estimatedCost:
 * this is what the workspace is actually charged. null when there is no bex
 * Subscription, Mode A excludes billing, Stripe is off, or the read degrades.
 */
export interface Billing {
  currentCost: BillingAmount | null;
  invoices: BillingInvoice[];
  credits: BillingCredits | null;
}

/**
 * How complete the metering behind `estimatedCost` is (ADR040). `state` is
 * "complete" | "partial" | "unknown": a degraded source turns the estimate
 * `partial` rather than failing the read, so a money figure built on incomplete
 * data can be caveated (w4/048). `through` bounds a partial estimate (RFC3339,
 * empty when unknown/complete) and `degradedSources` names the unhealthy meters.
 */
export interface Coverage {
  state: string;
  through: string;
  degradedSources: string[];
}

export interface UsageSummary {
  workspaceId: string;
  period: string; // "YYYY-MM"
  coverage: Coverage;
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
    skipPollAttempt: skipPollWhenHidden,
    fetchPolicy: PRIMED_FETCH_POLICY,
    errorPolicy: "all",
  });

  const raw = data?.usage;
  const summary = useMemo<UsageSummary | null>(
    () =>
      raw
        ? {
            workspaceId: raw.workspaceId ?? "",
            period: raw.period ?? "",
            coverage: {
              state: raw.coverage?.state ?? "unknown",
              through: raw.coverage?.through ?? "",
              degradedSources: (raw.coverage?.degradedSources ?? []).filter(
                Boolean,
              ),
            },
            estimatedCost: raw.estimatedCost
              ? {
                  totalUsd: raw.estimatedCost.totalUsd ?? "0.00",
                  resources: (raw.estimatedCost.resources ?? [])
                    .filter(Boolean)
                    .map((r) => ({
                      serviceId: r!.serviceId ?? "",
                      serviceName: r!.serviceName ?? "",
                      resourceKind: r!.resourceKind ?? "service",
                      costUsd: r!.costUsd ?? "0.00",
                      charges: (r!.charges ?? []).filter(Boolean).map((c) => ({
                        kind: c!.kind ?? "",
                        tier: c!.tier ?? "",
                        unit: c!.unit ?? "",
                        rateUsd: c!.rateUsd ?? "0",
                        quantity: c!.quantity ?? "0",
                        costUsd: c!.costUsd ?? "0.00",
                      })),
                    })),
                }
              : null,
            billing: raw.billing
              ? {
                  currentCost: raw.billing.currentCost
                    ? {
                        amountUsd: raw.billing.currentCost.amountUsd ?? "0.00",
                        creditsAppliedUsd:
                          raw.billing.currentCost.creditsAppliedUsd ?? "0.00",
                        amountDueUsd:
                          raw.billing.currentCost.amountDueUsd ?? "0.00",
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
                      creditsAppliedUsd: i!.creditsAppliedUsd ?? "0.00",
                      amountDueUsd: i!.amountDueUsd ?? "0.00",
                      currency: i!.currency ?? "USD",
                      periodStart: i!.periodStart ?? "",
                      periodEnd: i!.periodEnd ?? "",
                    })),
                  credits: raw.billing.credits
                    ? {
                        availableUsd:
                          raw.billing.credits.availableUsd ?? "0.00",
                        currency: raw.billing.credits.currency ?? "USD",
                        grants: (raw.billing.credits.grants ?? [])
                          .filter(Boolean)
                          .map((g) => ({
                            name: g!.name ?? "",
                            remainingUsd: g!.remainingUsd ?? "0.00",
                            expiresAt: g!.expiresAt ?? "",
                          })),
                      }
                    : null,
                }
              : null,
          }
        : null,
    [raw],
  );

  return { summary, loading: !resolved || loading, error };
}
