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
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { OrganizationPlanNote } from "@/features/workspaces/components/organization-plan-note";
import {
  WORKSPACE_PLAN_CATALOG,
  organizationPlanOwner,
} from "@/features/workspaces/types";

/**
 * The Billing page's pending cards (w4/707): each mirrors its ready card's
 * layout for the default view (the current month), with the card's own
 * translated copy laid out invisibly so it wraps as the ready copy does at
 * every width and in every language. What only the month's data decides stays
 * out: the test-mode badge, the lifecycle and operator banners, a cap tile's
 * near-limit and finishing-deletion notes, and an allowance's near/spent line.
 *
 * Skeleton-only, so the route's pending component can import it without
 * pulling the page's chunk up front (w3/066).
 */

/** A ready `CardTitle`'s line, with its own copy reserved. */
function ReservedTitle({ children }: { children: React.ReactNode }) {
  return (
    <CardTitle className="relative w-fit">
      <ReservedText>{children}</ReservedText>
    </CardTitle>
  );
}

/** A ready `CardDescription`, wrapping as its own copy does. */
function ReservedDescription({ children }: { children: React.ReactNode }) {
  return (
    <CardDescription className="relative">
      <ReservedText>{children}</ReservedText>
    </CardDescription>
  );
}

/**
 * Mirrors `PlanCard`: the plan box with the workspace's plan name and
 * description beside `Change plan`, which wraps under them on a phone. The
 * plan comes from the workspace list the shell already holds (every page load
 * sends it); until it arrives, Hobby, the plan a new workspace starts on,
 * stands in. A secondary workspace of an organization draws the org-plan note
 * in place of the button, as the ready card does.
 */
export function PlanCardSkeleton() {
  const { t } = useTranslations();
  const { currentWorkspace, workspaces } = useWorkspace();
  const planOwner = organizationPlanOwner(currentWorkspace, workspaces);
  const plan = currentWorkspace?.plan ?? "hobby";
  const entry = WORKSPACE_PLAN_CATALOG.find((p) => p.id === plan);

  return (
    <Card>
      <CardHeader>
        <ReservedTitle>{t("usage.planTitle")}</ReservedTitle>
      </CardHeader>
      <CardContent>
        <div className="flex flex-wrap items-start justify-between gap-4 rounded-md border p-4">
          <div className="min-w-0">
            <p className="relative w-fit font-medium">
              <ReservedText>{entry ? t(entry.nameKey) : plan}</ReservedText>
            </p>
            {entry ? (
              <p className="relative text-sm text-muted-foreground">
                <ReservedText>{t(entry.descriptionKey)}</ReservedText>
              </p>
            ) : null}
            {planOwner ? (
              <OrganizationPlanNote
                owner={planOwner}
                target="billing"
                pending
              />
            ) : null}
          </div>
          {planOwner ? null : (
            <ReservedButton variant="outline" size="sm">
              {t("usage.planChange")}
            </ReservedButton>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

/**
 * Mirrors `PaymentMethodCardView` for a workspace with a card on file: the
 * card line, `Update payment method` beside `Open billing portal` (stacking on
 * a phone), and the Stripe-hosted note.
 */
export function PaymentMethodCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <ReservedTitle>{t("usage.paymentMethodTitle")}</ReservedTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center gap-3 text-sm">
          <Skeleton className="size-5 shrink-0" />
          <Skeleton className="h-4 w-44" />
        </div>
        <div className="flex flex-wrap gap-2">
          <ReservedButton variant="outline">
            {t("usage.billingUpdatePayment")}
          </ReservedButton>
          <ReservedButton variant="outline">
            {t("usage.billingOpenPortal")}
          </ReservedButton>
        </div>
        <p className="relative text-xs text-muted-foreground">
          <ReservedText>{t("usage.billingHostedNote")}</ReservedText>
        </p>
      </CardContent>
    </Card>
  );
}

/**
 * The three cap tiles `ResourceCaps` draws on every plan (bex-api's
 * `QuotaCapsForPlan` caps all three kinds everywhere). Postgres and Key Value
 * carry the one-free-instance line: bex-api reports a free limit of 1 for both
 * on every plan (w4/m241), and none for services.
 */
const CAP_TILES = [
  { label: "usage.resourceCapsServices", free: false },
  { label: "usage.resourceCapsPostgres", free: true },
  { label: "usage.resourceCapsKeyValues", free: true },
] as const;

/** Mirrors `ResourceCaps`: three tiles in one row, stacking on a phone. */
function ResourceCapsSkeleton() {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <ReservedTitle>{t("usage.includedUsageTitle")}</ReservedTitle>
        <ReservedDescription>
          {t("usage.includedUsageDescription")}
        </ReservedDescription>
      </CardHeader>
      <CardContent className="grid gap-3 sm:grid-cols-3">
        {CAP_TILES.map(({ label, free }) => (
          <div key={label} className="overflow-hidden rounded-lg border">
            <div className="space-y-1 p-3">
              <p className="relative w-fit text-sm text-muted-foreground">
                <ReservedText>{t(label)}</ReservedText>
              </p>
              {/* The count's `text-xl` line. */}
              <div className="flex h-7 items-center">
                <Skeleton className="h-6 w-16" />
              </div>
              {free ? (
                <p className="relative w-fit text-xs text-muted-foreground">
                  <ReservedText>
                    {t("usage.resourceCapsFree", { used: 0, limit: 1 })}
                  </ReservedText>
                </p>
              ) : null}
            </div>
            <div className="h-0.5 bg-muted" />
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

/**
 * Mirrors `IncludedUsageCard` below its near/spent line: the title, the
 * description, the used line and the meter. `used` is the ready line with
 * representative figures, so it wraps where the ready one does.
 */
function IncludedUsageCardSkeleton({
  title,
  description,
  used,
}: {
  title: string;
  description: string;
  used: string;
}) {
  return (
    <Card>
      <CardHeader>
        <ReservedTitle>{title}</ReservedTitle>
        <ReservedDescription>{description}</ReservedDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <p className="relative w-fit text-sm font-medium">
          <ReservedText>{used}</ReservedText>
        </p>
        <Skeleton className="h-2 w-full rounded-full" />
      </CardContent>
    </Card>
  );
}

/**
 * The included-usage section's cards for the current month, in the ready
 * order: the caps, Free instance hours, pipeline minutes and outbound
 * bandwidth on every plan, then custom domains on every plan but Enterprise,
 * whose domains are all included (`CustomDomainsCard`). The plan comes from
 * the workspace list the shell already holds.
 *
 * Each used line reserves the ready sentence with an active workspace's
 * figures, formatted as the ready card formats them (hours to one decimal,
 * gigabytes to two). The figures are the month's data, and on a phone the
 * line sits near its wrap point, so a workspace whose figures are much
 * shorter or longer can wrap one line differently.
 */
export function IncludedUsageSkeleton() {
  const { t } = useTranslations();
  const { currentWorkspace } = useWorkspace();
  const allowances = [
    {
      key: "free-hours",
      title: t("usage.freeHoursTitle"),
      description: t("usage.freeHoursDescription"),
      used: t("usage.freeHoursUsed", { used: "10.0", included: "750" }),
    },
    {
      key: "build-minutes",
      title: t("usage.buildMinutesTitle"),
      description: t("usage.buildMinutesDescription"),
      used: t("usage.buildMinutesUsed", { used: "100", included: "1000" }),
    },
    {
      key: "bandwidth",
      title: t("usage.bandwidthTitle"),
      description: t("usage.bandwidthDescription"),
      used: t("usage.bandwidthUsed", { used: "1.00", included: "25" }),
    },
    ...(currentWorkspace?.plan === "enterprise"
      ? []
      : [
          {
            key: "custom-domains",
            title: t("usage.customDomainsTitle"),
            description: t("usage.customDomainsDescription"),
            used: t("usage.customDomainsUsed", { used: "1", included: "15" }),
          },
        ]),
  ];
  return (
    <>
      <ResourceCapsSkeleton />
      {allowances.map(({ key, ...card }) => (
        <IncludedUsageCardSkeleton key={key} {...card} />
      ))}
    </>
  );
}

/**
 * Mirrors `CreditBalanceCard` for a workspace holding credit: the total's
 * label and amount, the credit-applied line and the card-still-required note.
 */
export function CreditBalanceCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <ReservedTitle>{t("usage.creditsTitle")}</ReservedTitle>
        <ReservedDescription>
          {t("usage.creditsDescription")}
        </ReservedDescription>
      </CardHeader>
      <CardContent>
        <p className="relative w-fit text-xs tracking-wide text-muted-foreground uppercase">
          <ReservedText>{t("usage.creditsTotalLabel")}</ReservedText>
        </p>
        {/* The amount's `text-3xl` line. */}
        <div className="flex h-9 items-center">
          <Skeleton className="h-8 w-40" />
        </div>
        <p className="relative mt-1 w-fit text-sm text-muted-foreground">
          <ReservedText>
            {t("usage.creditsAppliedLine", { applied: "$0.00", due: "$0.00" })}
          </ReservedText>
        </p>
        <p className="relative mt-2 text-xs text-muted-foreground">
          <ReservedText>{t("usage.creditsCardStillRequired")}</ReservedText>
        </p>
      </CardContent>
    </Card>
  );
}
