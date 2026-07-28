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

import { CircleCheck, CircleDashed, TriangleAlert } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useBillingOnboarding,
  type BillingReadiness,
} from "../hooks/use-billing-onboarding";

interface BillingOnboardingViewProps {
  readiness: BillingReadiness | null;
  loading: boolean;
  error?: Error;
  checkoutBusy: boolean;
  portalBusy: boolean;
  onCheckout: () => void;
  onPortal: () => void;
}

function StatusRow({ label, ready }: { label: string; ready: boolean }) {
  const { t } = useTranslations();
  return (
    <div className="flex items-center justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="flex items-center gap-1.5 font-medium">
        {ready ? (
          <CircleCheck aria-hidden="true" className="size-4 text-emerald-600" />
        ) : (
          <CircleDashed aria-hidden="true" className="size-4 text-amber-600" />
        )}
        {ready ? t("usage.billingReady") : t("usage.billingActionNeeded")}
      </span>
    </div>
  );
}

function LifecycleAlert({ readiness }: { readiness: BillingReadiness }) {
  const { t } = useTranslations();
  const state = readiness.lifecycle;
  if (state.status === "healthy") return null;
  let key = "usage.billingLifecycleUnknown";
  if (state.status === "grace") key = "usage.billingLifecycleGrace";
  if (state.status === "enforcing" || state.status === "enforced")
    key = "usage.billingLifecycleEnforced";
  if (state.status === "recovering") key = "usage.billingLifecycleRecovering";
  if (state.status === "excluded") key = "usage.billingLifecycleExcluded";
  if (state.status === "comped") key = "usage.billingLifecycleComped";
  return (
    <Alert variant={state.enforcementOwned ? "destructive" : "default"}>
      <TriangleAlert aria-hidden="true" />
      <AlertDescription>
        {t(key, {
          deadline: state.graceDeadline || t("usage.billingNoDeadline"),
          reason: state.reason || state.status,
        })}
      </AlertDescription>
    </Alert>
  );
}

export function BillingOnboardingView({
  readiness,
  loading,
  error,
  checkoutBusy,
  portalBusy,
  onCheckout,
  onPortal,
}: BillingOnboardingViewProps) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle>{t("usage.billingSetupTitle")}</CardTitle>
          <Badge variant={readiness?.mode === "test" ? "secondary" : "default"}>
            {readiness?.mode === "test"
              ? t("usage.billingTestMode")
              : t("usage.billingMode")}
          </Badge>
        </div>
        <CardDescription>{t("usage.billingSetupDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading && !readiness ? (
          <div className="space-y-2">
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-5 w-full" />
          </div>
        ) : error || !readiness ? (
          <Alert variant="destructive">
            <TriangleAlert aria-hidden="true" />
            <AlertDescription>{t("usage.billingUnavailable")}</AlertDescription>
          </Alert>
        ) : (
          <>
            <div className="divide-y rounded-md border px-3">
              <StatusRow
                label={t("usage.billingCustomerStatus")}
                ready={readiness.customerReady}
              />
              <StatusRow
                label={t("usage.billingSubscriptionStatus")}
                ready={readiness.subscriptionReady}
              />
              <StatusRow
                label={t("usage.billingPaymentStatus")}
                ready={readiness.paymentMethodReady}
              />
              <StatusRow
                label={t("usage.billingTaxStatus")}
                ready={readiness.tax.enabled}
              />
              <StatusRow
                label={t("usage.billingLifecycleStatus")}
                ready={readiness.lifecycle.status === "healthy"}
              />
            </div>
            <LifecycleAlert readiness={readiness} />
            {!readiness.tax.configured && (
              <Alert>
                <TriangleAlert aria-hidden="true" />
                <AlertDescription>
                  {t("usage.billingTaxUnconfigured")}
                </AlertDescription>
              </Alert>
            )}
            <div className="flex flex-wrap gap-2">
              <Button loading={checkoutBusy} onClick={onCheckout}>
                {readiness.paymentMethodReady
                  ? t("usage.billingUpdatePayment")
                  : t("usage.billingAddPayment")}
              </Button>
              <Button
                variant="outline"
                loading={portalBusy}
                disabled={
                  !readiness.customerReady || !readiness.subscriptionReady
                }
                onClick={onPortal}
              >
                {t("usage.billingOpenPortal")}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              {t("usage.billingHostedNote")}
            </p>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export function BillingOnboardingCard() {
  const state = useBillingOnboarding();
  return (
    <BillingOnboardingView
      readiness={state.readiness}
      loading={state.loading}
      error={state.error}
      checkoutBusy={state.checkoutBusy}
      portalBusy={state.portalBusy}
      onCheckout={() => void state.openCheckout()}
      onPortal={() => void state.openPortal()}
    />
  );
}
