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

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { UsageMeter } from "@/common/components/usage-meter";
import { useTranslations } from "@/common/hooks/use-translations";
import type { FreeInstanceHours } from "@/features/usage/hooks/use-usage";

const NEAR_LIMIT_RATIO = 0.8;

/**
 * Render's "Monthly included usage" for Free web services (w8/m139): the
 * workspace's Free instance hours against the monthly allowance. Free web
 * services are suspended until next month once it is spent.
 */
export function FreeHoursCard({ hours }: { hours: FreeInstanceHours | null }) {
  const { t } = useTranslations();
  if (!hours) return null;
  const ratio = hours.included > 0 ? hours.used / hours.included : 0;
  const spent = ratio >= 1;
  const near = ratio >= NEAR_LIMIT_RATIO;
  const usedText = t("usage.freeHoursUsed", {
    used: hours.used.toFixed(1),
    included: String(hours.included),
  });
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.freeHoursTitle")}</CardTitle>
        <CardDescription>{t("usage.freeHoursDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <p className="text-sm font-medium">{usedText}</p>
        <UsageMeter
          label={t("usage.freeHoursTitle")}
          fraction={ratio}
          level={spent ? "reached" : near ? "warning" : "ok"}
          valueText={usedText}
        />
        {spent ? (
          <p className="text-xs text-destructive">
            {t("usage.freeHoursSpent")}
          </p>
        ) : near ? (
          <p className="text-xs text-muted-foreground">
            {t("usage.freeHoursNear")}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
