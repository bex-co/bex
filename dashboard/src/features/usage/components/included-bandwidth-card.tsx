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

import { useTranslations } from "@/common/hooks/use-translations";
import type { IncludedBandwidth } from "@/features/usage/hooks/use-usage";
import { IncludedUsageCard } from "./included-usage-card";

/**
 * Render's included outbound bandwidth (w6/m319, docs/ADR030 §14): the month's
 * outbound bandwidth against what the workspace plan includes. Bandwidth beyond
 * it is billed; without a payment method the workspace's services spin down
 * until next month instead. Nothing renders where no plan includes any.
 */
export function IncludedBandwidthCard({
  bandwidth,
}: {
  bandwidth: IncludedBandwidth | null;
}) {
  const { t } = useTranslations();
  if (!bandwidth || bandwidth.included <= 0) return null;
  return (
    <IncludedUsageCard
      title={t("usage.bandwidthTitle")}
      description={t("usage.bandwidthDescription")}
      usedText={t("usage.bandwidthUsed", {
        used: formatGB(bandwidth.used),
        included: String(bandwidth.included),
      })}
      ratio={bandwidth.used / bandwidth.included}
      nearText={t("usage.bandwidthNear")}
      spentText={t("usage.bandwidthSpent")}
    />
  );
}

// formatGB keeps a few MB of traffic from reading as zero.
function formatGB(gb: number): string {
  if (gb === 0) return "0";
  return gb < 0.01 ? "<0.01" : gb.toFixed(2);
}
