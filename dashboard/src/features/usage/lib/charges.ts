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

import type { ResourceEstimate } from "../hooks/use-usage";

/** The arithmetic behind the billing page's charge tree, kept out of the view. */

/** Fixed category order, so the tree does not reshuffle as usage appears. */
export const CATEGORY_ORDER = [
  "service",
  "postgres",
  "key_value",
  "sandbox",
] as const;

export interface ChargeCategory {
  key: string;
  resources: ResourceEstimate[];
  totalUsd: number;
}

/** Display-side USD math on the backend's normalized "12.34" strings. */
export function usd(amount: string): number {
  const n = Number.parseFloat(amount);
  return Number.isFinite(n) ? n : 0;
}

export function money(value: number): string {
  return `$${value.toFixed(2)}`;
}

/** "YYYY-MM" for a given instant — the period the charge tree treats as live. */
export function currentPeriod(now: Date): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

/** Group resources into categories, keeping CATEGORY_ORDER then anything new. */
export function groupByCategory(
  resources: ResourceEstimate[],
): ChargeCategory[] {
  const byKind = new Map<string, ResourceEstimate[]>();
  for (const r of resources) {
    const key = r.resourceKind || "service";
    const bucket = byKind.get(key);
    if (bucket) bucket.push(r);
    else byKind.set(key, [r]);
  }
  const keys = [
    ...CATEGORY_ORDER.filter((k) => byKind.has(k)),
    // A resource kind the frontend has not been taught yet still bills, so it
    // must appear rather than silently vanish from the total.
    ...[...byKind.keys()].filter(
      (k) => !CATEGORY_ORDER.includes(k as (typeof CATEGORY_ORDER)[number]),
    ),
  ];
  return keys.map((key) => {
    const group = byKind.get(key)!;
    return {
      key,
      resources: [...group].sort((a, b) => usd(b.costUsd) - usd(a.costUsd)),
      totalUsd: group.reduce((sum, r) => sum + usd(r.costUsd), 0),
    };
  });
}

/**
 * Linear month-to-date projection: spend so far, scaled by how much of the
 * month remains. Only meaningful while the month is still running, so callers
 * pass null for a historical period.
 */
export function projectMonthEnd(spentUsd: number, now: Date): number | null {
  const start = new Date(now.getFullYear(), now.getMonth(), 1).getTime();
  const end = new Date(now.getFullYear(), now.getMonth() + 1, 1).getTime();
  const elapsed = now.getTime() - start;
  // Guard the first moments of a month, where the ratio explodes.
  if (elapsed <= 0 || spentUsd <= 0) return null;
  return (spentUsd * (end - start)) / elapsed;
}
