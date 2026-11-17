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
import type { CustomDomainsUsage } from "@/features/usage/hooks/use-usage";
import { IncludedUsageCard } from "./included-usage-card";

/**
 * Render's included custom domains (w7/m429, docs/ADR030 §16): the verified
 * custom domains now against the count the workspace plan includes. Each one
 * beyond it is billed per domain-month (the charge tree's custom domain line),
 * and none is refused for the count. Nothing renders for a past month, where
 * no plan applies, or on Enterprise, whose domains are all included.
 */
export function CustomDomainsCard({
  domains,
}: {
  domains: CustomDomainsUsage | null;
}) {
  const { t } = useTranslations();
  if (!domains || domains.unlimited || domains.included <= 0) return null;
  return (
    <IncludedUsageCard
      title={t("usage.customDomainsTitle")}
      description={t("usage.customDomainsDescription")}
      usedText={t("usage.customDomainsUsed", {
        used: String(domains.used),
        included: String(domains.included),
      })}
      ratio={domains.used / domains.included}
      nearText={t("usage.customDomainsNear")}
      spentText={t("usage.customDomainsSpent")}
    />
  );
}
