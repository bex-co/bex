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

import { useEffect } from "react";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { CreditCard, Github, Loader2, TriangleAlert } from "lucide-react";
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
import { safeNext } from "@/common/lib/safe-next";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useBillingOnboarding } from "@/features/usage/hooks/use-billing-onboarding";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  paymentSetupPath,
  paymentSetupState,
  SELF_HOST_URL,
} from "../../lib/payment-setup";

/**
 * The sign-up payment wall (`/setup/payment`, ADR075 D7 revised 2026-08-29):
 * the one step between a verified sign-up and the product. Hosted bex is a
 * paid product, so a workspace the `all` gate still refuses cannot run
 * anything — this page collects the card up front instead of waiting for the
 * first create to 402 into the interception dialog (which stays as the
 * backstop for API callers and non-manager members).
 *
 * Same-tab Stripe Checkout returns here (`?billing=success|cancelled`); the
 * wall then polls readiness at the dialog's 2s cadence until the signed
 * webhook commits the marker (Stripe's success redirect alone is not proof),
 * and continues to the guarded `next` the moment the server says the gate is
 * open. Exits that are not a dead end: self-hosting (the free path) and sign
 * out. A caller who cannot bind a card here (not a billing manager) or a
 * workspace that no longer needs one is forwarded straight through.
 */
export default function PaymentSetupPage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const search = useSearch({ from: "/setup/payment" });
  // `next` is attacker-controllable (it's in the URL): normalize before it is
  // navigated to or echoed into Stripe's return URL (safe-next.ts).
  const next = safeNext(search.next);
  const capabilities = useCapabilities();
  const billingForbidden =
    capabilities.loaded && !capabilities.canManageBilling;
  const { currentWorkspace } = useWorkspace();
  const billing = useBillingOnboarding({
    active: !billingForbidden,
    pollInterval: 2_000,
    returnPath: paymentSetupPath(next),
  });
  const state = paymentSetupState({
    readiness: billing.readiness,
    loading: billing.loading,
    error: billing.error,
    billingForbidden,
  });
  const readiness = billing.readiness;
  const testMode = readiness?.mode === "test";

  useEffect(() => {
    if (state !== "satisfied" && state !== "forbidden") return;
    // `next` goes in `href`, not `to` (see login-page): it may carry a query
    // string. Already same-origin-normalized above.
    void navigate({ to: "/", href: next, replace: true });
  }, [state, next, navigate]);

  return (
    <AuthPageShell
      title={t("onboarding.paymentSetupTitle")}
      subtitle={t("onboarding.paymentSetupSubtitle")}
    >
      <Card data-payment-setup-state={state}>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-2">
            <CardTitle>{t("onboarding.paymentSetupCardTitle")}</CardTitle>
            {testMode && (
              <Badge variant="secondary">{t("usage.billingTestMode")}</Badge>
            )}
          </div>
          <CardDescription>{t("onboarding.paymentSetupBody")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {state === "required" ? (
            <>
              {currentWorkspace ? (
                <p className="text-sm text-muted-foreground">
                  {t("onboarding.paymentSetupWorkspace", {
                    name: currentWorkspace.name,
                  })}
                </p>
              ) : null}
              {search.billing === "success" ? (
                <p
                  className="flex items-center gap-2 text-sm text-muted-foreground"
                  role="status"
                >
                  <Loader2
                    aria-hidden="true"
                    className="size-4 shrink-0 animate-spin"
                  />
                  {t("onboarding.paymentSetupConfirming")}
                </p>
              ) : null}
              {search.billing === "cancelled" ? (
                <Alert>
                  <TriangleAlert aria-hidden="true" />
                  <AlertDescription>
                    {t("onboarding.paymentSetupCancelled")}
                  </AlertDescription>
                </Alert>
              ) : null}
              <Button
                className="w-full"
                size="lg"
                loading={billing.checkoutBusy}
                onClick={() => void billing.openCheckout()}
              >
                <CreditCard aria-hidden="true" />
                {t(
                  testMode
                    ? "usage.billingAddPaymentTest"
                    : "usage.billingAddPayment",
                )}
              </Button>
              <p className="text-xs text-muted-foreground">
                {t("usage.billingHostedNote")}
              </p>
            </>
          ) : state === "unavailable" ? (
            <>
              <Alert variant="destructive">
                <TriangleAlert aria-hidden="true" />
                <AlertDescription>
                  {t("usage.billingUnavailable")}
                </AlertDescription>
              </Alert>
              <div className="flex flex-wrap gap-2">
                <Button onClick={() => void billing.refetch()}>
                  {t("onboarding.paymentSetupRetry")}
                </Button>
                <Button variant="outline" asChild>
                  <Link to="/" href={next}>
                    {t("onboarding.paymentSetupContinue")}
                  </Link>
                </Button>
              </div>
            </>
          ) : (
            // loading, or satisfied/forbidden while the forward navigation
            // is in flight — the same geometry as the skeleton this route
            // paints while pending, so nothing jumps.
            <div className="space-y-3" aria-busy="true">
              <span className="sr-only" role="status">
                {t("onboarding.paymentSetupContinuing")}
              </span>
              <Skeleton className="h-5 w-2/3" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-4 w-4/5" />
            </div>
          )}
          <div className="space-y-2 border-t pt-4 text-sm">
            <p className="text-muted-foreground">
              {t("onboarding.paymentSetupSelfHostHint")}
            </p>
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
              <a
                href={SELF_HOST_URL}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1.5 font-medium text-primary underline-offset-4 hover:underline"
              >
                <Github aria-hidden="true" className="size-4" />
                {t("onboarding.paymentSetupSelfHost")}
              </a>
              <Link
                to="/auth/logout"
                className="text-muted-foreground underline-offset-4 hover:underline"
              >
                {t("onboarding.paymentSetupSignOut")}
              </Link>
            </div>
          </div>
        </CardContent>
      </Card>
    </AuthPageShell>
  );
}
