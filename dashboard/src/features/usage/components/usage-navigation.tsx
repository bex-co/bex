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
  ArrowDownUp,
  Cpu,
  CreditCard,
  Gauge,
  Hammer,
  HardDrive,
  CircleDollarSign,
  TrendingUp,
} from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * In-page section navigation for the billing page (`/billing`, formerly
 * `/usage`) — same right-rail quick nav as the service settings page. Anchors
 * point only at always-rendered elements (the two section headings and the
 * unconditional cards), never at the conditional credits/spend/caps cards.
 */
export function UsageNavigation({ className }: { className?: string }) {
  const { t } = useTranslations();
  const items = [
    { href: "#billing", label: t("usage.sectionBilling"), icon: CreditCard },
    {
      href: "#estimated-cost",
      label: t("usage.estimatedCostTitle"),
      icon: CircleDollarSign,
    },
    { href: "#usage", label: t("usage.sectionUsage"), icon: Gauge },
    { href: "#compute", label: t("usage.computeTitle"), icon: Cpu },
    { href: "#bandwidth", label: t("usage.bandwidthTitle"), icon: ArrowDownUp },
    { href: "#build", label: t("usage.buildTitle"), icon: Hammer },
    { href: "#storage", label: t("usage.storageTitle"), icon: HardDrive },
    { href: "#trend", label: t("usage.trendTitle"), icon: TrendingUp },
  ];

  return (
    <SectionNavigation
      ariaLabel={t("usage.sectionNavigation")}
      items={items}
      className={className}
    />
  );
}
