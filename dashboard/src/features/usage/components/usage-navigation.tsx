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
  CreditCard,
  FileText,
  Gauge,
  Receipt,
  Sparkles,
  Wallet,
} from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * In-page section navigation for the billing page — same right-rail quick nav
 * as the service settings page. One entry per card, in page order: the nav is
 * a map of the page, so an entry that points at something conditional (credit
 * balance, invoice history) is still correct — those sections keep their
 * anchors and simply render nothing when they do not apply.
 */
export function UsageNavigation({ className }: { className?: string }) {
  const { t } = useTranslations();
  const items = [
    { href: "#plan", label: t("usage.planTitle"), icon: Sparkles },
    {
      href: "#payment-method",
      label: t("usage.paymentMethodTitle"),
      icon: CreditCard,
    },
    {
      href: "#included-usage",
      label: t("usage.includedUsageTitle"),
      icon: Gauge,
    },
    { href: "#charges", label: t("usage.chargesTitle"), icon: Receipt },
    { href: "#credit-balance", label: t("usage.creditsTitle"), icon: Wallet },
    {
      href: "#invoice-history",
      label: t("usage.invoiceHistoryTitle"),
      icon: FileText,
    },
  ];

  return (
    <SectionNavigation
      ariaLabel={t("usage.sectionNavigation")}
      items={items}
      className={className}
    />
  );
}
