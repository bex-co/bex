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
import type { FreeInstanceHours } from "@/features/usage/hooks/use-usage";
import { IncludedUsageCard } from "./included-usage-card";

/**
 * Render's "Monthly included usage" for Free web services (w8/m139): the
 * workspace's Free instance hours against the monthly allowance. Free web
 * services are suspended until next month once it is spent.
 */
export function FreeHoursCard({ hours }: { hours: FreeInstanceHours | null }) {
  const { t } = useTranslations();
  if (!hours) return null;
  return (
    <IncludedUsageCard
      title={t("usage.freeHoursTitle")}
      description={t("usage.freeHoursDescription")}
      usedText={t("usage.freeHoursUsed", {
        used: hours.used.toFixed(1),
        included: String(hours.included),
      })}
      ratio={hours.included > 0 ? hours.used / hours.included : 0}
      nearText={t("usage.freeHoursNear")}
      spentText={t("usage.freeHoursSpent")}
    />
  );
}
