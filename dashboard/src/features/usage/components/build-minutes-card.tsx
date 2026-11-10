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
import type { BuildMinutes } from "@/features/usage/hooks/use-usage";
import { IncludedUsageCard } from "./included-usage-card";

/**
 * Render's included pipeline minutes (w6/m308, docs/ADR030 §13): the month's
 * build minutes against the minutes the workspace plan includes. Minutes
 * beyond them are billed; without a payment method builds stop until next
 * month instead. Nothing renders where no plan includes any.
 */
export function BuildMinutesCard({
  minutes,
}: {
  minutes: BuildMinutes | null;
}) {
  const { t } = useTranslations();
  if (!minutes || minutes.included <= 0) return null;
  return (
    <IncludedUsageCard
      title={t("usage.buildMinutesTitle")}
      description={t("usage.buildMinutesDescription")}
      usedText={t("usage.buildMinutesUsed", {
        used: minutes.used.toFixed(0),
        included: String(minutes.included),
      })}
      ratio={minutes.used / minutes.included}
      nearText={t("usage.buildMinutesNear")}
      spentText={t("usage.buildMinutesSpent")}
    />
  );
}
