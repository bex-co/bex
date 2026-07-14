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
import type { ServiceUsage } from "@/graphql/definitions";
import type { Maybe } from "@/graphql/definitions";
import { periodFor } from "@/features/usage/lib/period";

/** One month's aggregated totals per meter kind. */
export interface TrendPoint {
  period: string; // "YYYY-MM"
  timestamp: string; // ISO string of the 1st of the month — x-axis for SvgLineChart
  compute: number; // total instance_seconds
  bandwidth: number; // total egress_bytes
  build: number; // total build_seconds
  storage: number; // total storage_gb_seconds
}

const TREND_QUERY_OPTS = {
  fetchPolicy: "cache-and-network" as const,
  errorPolicy: "all" as const,
};

function sumKind(
  services: Maybe<Array<Maybe<ServiceUsage>>> | undefined,
  kind: string,
): number {
  let total = 0;
  for (const svc of services?.filter(Boolean) ?? []) {
    for (const row of svc?.rows?.filter(Boolean) ?? []) {
      if (row?.kind === kind) total += row.total ?? 0;
    }
  }
  return total;
}

/**
 * Fetches usage for the last three calendar months (current + 2 prior) and
 * returns one TrendPoint per month, sorted oldest-first, ready for SvgLineChart.
 */
export function useUsageTrend(): { points: TrendPoint[]; loading: boolean } {
  const p0 = periodFor(0); // current month
  const p1 = periodFor(1); // 1 month ago
  const p2 = periodFor(2); // 2 months ago

  const r0 = useQuery(UsageDocument, {
    variables: { period: p0 },
    ...TREND_QUERY_OPTS,
  });
  const r1 = useQuery(UsageDocument, {
    variables: { period: p1 },
    ...TREND_QUERY_OPTS,
  });
  const r2 = useQuery(UsageDocument, {
    variables: { period: p2 },
    ...TREND_QUERY_OPTS,
  });

  const loading = r0.loading || r1.loading || r2.loading;

  const points = useMemo<TrendPoint[]>(() => {
    return [
      { period: p2, services: r2.data?.usage?.services },
      { period: p1, services: r1.data?.usage?.services },
      { period: p0, services: r0.data?.usage?.services },
    ].map(({ period, services }) => {
      const [year, month] = period.split("-").map(Number);
      return {
        period,
        timestamp: new Date(year!, month! - 1, 1).toISOString(),
        compute: sumKind(services, "instance_seconds"),
        bandwidth: sumKind(services, "egress_bytes"),
        build: sumKind(services, "build_seconds"),
        storage: sumKind(services, "storage_gb_seconds"),
      };
    });
  }, [r0.data, r1.data, r2.data, p0, p1, p2]);

  return { points, loading };
}
