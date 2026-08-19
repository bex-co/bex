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

import { CreditCard, TriangleAlert } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import {
  useBillingOnboarding,
  type BillingReadiness,
} from "../hooks/use-billing-onboarding";

/**
 * The payment method on file, plus the two hosted actions that change it.
 *
 * This replaced a five-row readiness checklist (Customer / Metered
 * subscription / Payment method / Automatic tax / Collection lifecycle) that
 * exposed bex's Stripe object graph to end users: of those five rows exactly
 * one — is there a card — is a fact a customer can act on, and the other four
 * read as broken setup on a perfectly healthy workspace. The full checklist
 * still exists where it earns its place: the paid-intent dialog
 * (`BillingOnboardingView`), which has to explain precisely why a create was
 * refused. Anything genuinely wrong here surfaces as the lifecycle alert.
 */

/** Stripe brand ids are lowercase slugs; these are their display names. */
const BRAND_LABELS: Record<string, string> = {
  amex: "American Express",
  diners: "Diners Club",
  discover: "Discover",
  eftpos_au: "Eftpos Australia",
  jcb: "JCB",
  mastercard: "Mastercard",
  unionpay: "UnionPay",
  visa: "Visa",
};

function brandLabel(brand: string): string {
  if (!brand) return "";
  return (
    BRAND_LABELS[brand] ??
    // Unknown brand: title-case the slug rather than print "cartes_bancaires".
    brand.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())
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

export interface PaymentMethodCardViewProps {
  readiness: BillingReadiness | null;
  loading: boolean;
  error?: Error;
  checkoutBusy: boolean;
  portalBusy: boolean;
  onCheckout: () => void;
  onPortal: () => void;
  canManageBilling: boolean;
}

export function PaymentMethodCardView({
  readiness,
  loading,
  error,
  checkoutBusy,
  portalBusy,
  onCheckout,
  onPortal,
  canManageBilling,
}: PaymentMethodCardViewProps) {
  const { t } = useTranslations();
  const reason = canManageBilling
    ? undefined
    : t("capabilities.reasonCanManageBilling");
  const testMode = readiness?.mode === "test";
  const brand = brandLabel(readiness?.paymentMethodBrand ?? "");
  const last4 = readiness?.paymentMethodLast4 ?? "";

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle>{t("usage.paymentMethodTitle")}</CardTitle>
          {testMode && (
            <Badge variant="secondary">{t("usage.billingTestMode")}</Badge>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading && !readiness ? (
          <Skeleton className="h-10 w-64" />
        ) : error || !readiness ? (
          <Alert variant="destructive">
            <TriangleAlert aria-hidden="true" />
            <AlertDescription>{t("usage.billingUnavailable")}</AlertDescription>
          </Alert>
        ) : (
          <>
            <div className="flex items-center gap-3 text-sm">
              <CreditCard
                aria-hidden="true"
                className="size-5 shrink-0 text-muted-foreground"
              />
              {!readiness.paymentMethodReady ? (
                <span className="text-muted-foreground">
                  {t("usage.paymentMethodNone")}
                </span>
              ) : brand && last4 ? (
                <span>{t("usage.paymentMethodCard", { brand, last4 })}</span>
              ) : (
                // A non-card method, or a read that could not expand it.
                <span>{t("usage.paymentMethodOnFile")}</span>
              )}
            </div>
            <LifecycleAlert readiness={readiness} />
            <div className="flex flex-wrap gap-2">
              <PermissionTooltip reason={reason}>
                <Button
                  loading={checkoutBusy}
                  disabled={!canManageBilling}
                  onClick={onCheckout}
                  variant={readiness.paymentMethodReady ? "outline" : "default"}
                >
                  {readiness.paymentMethodReady
                    ? t("usage.billingUpdatePayment")
                    : t("usage.billingAddPayment")}
                </Button>
              </PermissionTooltip>
              <PermissionTooltip reason={reason}>
                <Button
                  variant="outline"
                  loading={portalBusy}
                  disabled={
                    !canManageBilling ||
                    !readiness.customerReady ||
                    !readiness.subscriptionReady
                  }
                  onClick={onPortal}
                >
                  {t("usage.billingOpenPortal")}
                </Button>
              </PermissionTooltip>
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

export function PaymentMethodCard() {
  const state = useBillingOnboarding();
  const { canManageBilling } = useCapabilities();
  return (
    <PaymentMethodCardView
      readiness={state.readiness}
      loading={state.loading}
      error={state.error}
      checkoutBusy={state.checkoutBusy}
      portalBusy={state.portalBusy}
      onCheckout={() => void state.openCheckout()}
      onPortal={() => void state.openPortal()}
      canManageBilling={canManageBilling}
    />
  );
}
