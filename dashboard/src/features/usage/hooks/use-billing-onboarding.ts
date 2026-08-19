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

import { useCallback, useMemo, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { toast } from "sonner";
import {
  BillingReadinessDocument,
  CreateBillingCheckoutSessionDocument,
  CreateBillingPortalSessionDocument,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useTranslations } from "@/common/hooks/use-translations";

export interface BillingTaxReadiness {
  configured: boolean;
  enabled: boolean;
  reason: string;
  productTaxCode: string;
  taxBehavior: string;
  registrationCount: number;
}

export interface BillingReadiness {
  workspaceId: string;
  mode: string;
  customerReady: boolean;
  subscriptionReady: boolean;
  paymentMethodReady: boolean;
  /** Card brand + last four of the method Stripe will charge; both empty for a
   *  non-card method or a degraded read, in which case only the flag is known. */
  paymentMethodBrand: string;
  paymentMethodLast4: string;
  lifecycle: BillingLifecycle;
  tax: BillingTaxReadiness;
}

export interface BillingLifecycle {
  status: string;
  reason: string;
  graceDeadline: string;
  enforcementOwned: boolean;
  recoveryPending: boolean;
  allowedActions: string[];
  updatedAt: string;
}

export interface UseBillingOnboardingResult {
  readiness: BillingReadiness | null;
  loading: boolean;
  error: Error | undefined;
  checkoutBusy: boolean;
  portalBusy: boolean;
  openCheckout: () => Promise<void>;
  openPortal: () => Promise<void>;
  refetch: () => Promise<unknown>;
}

export interface UseBillingOnboardingOptions {
  active?: boolean;
  pollInterval?: number;
  checkoutTarget?: "same-tab" | "new-tab";
}

function billingReturnURL(state?: "success" | "cancelled"): string {
  const url = new URL("/billing", window.location.origin);
  if (state) url.searchParams.set("billing", state);
  return url.toString();
}

export function useBillingOnboarding({
  active = true,
  pollInterval = 15_000,
  checkoutTarget = "same-tab",
}: UseBillingOnboardingOptions = {}): UseBillingOnboardingResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(BillingReadinessDocument, {
    variables: { workspaceId: currentWorkspaceId ?? "" },
    skip: !active || !resolved,
    fetchPolicy: "cache-and-network",
    pollInterval: active ? pollInterval : 0,
    skipPollAttempt: skipPollWhenHidden,
    errorPolicy: "all",
  });
  const [createCheckout] = useMutation(CreateBillingCheckoutSessionDocument, {
    fetchPolicy: "no-cache",
  });
  const [createPortal] = useMutation(CreateBillingPortalSessionDocument, {
    fetchPolicy: "no-cache",
  });
  const [checkoutBusy, setCheckoutBusy] = useState(false);
  const [portalBusy, setPortalBusy] = useState(false);

  const readiness = useMemo<BillingReadiness | null>(() => {
    const raw = data?.workspaceBillingReadiness;
    if (!raw) return null;
    return {
      workspaceId: raw.workspaceId ?? "",
      mode: raw.mode ?? "",
      customerReady: raw.customerReady ?? false,
      subscriptionReady: raw.subscriptionReady ?? false,
      paymentMethodReady: raw.paymentMethodReady ?? false,
      paymentMethodBrand: raw.paymentMethodBrand ?? "",
      paymentMethodLast4: raw.paymentMethodLast4 ?? "",
      lifecycle: {
        status: raw.lifecycle?.status ?? "healthy",
        reason: raw.lifecycle?.reason ?? "",
        graceDeadline: raw.lifecycle?.graceDeadline ?? "",
        enforcementOwned: raw.lifecycle?.enforcementOwned ?? false,
        recoveryPending: raw.lifecycle?.recoveryPending ?? false,
        allowedActions: raw.lifecycle?.allowedActions?.filter(
          (action): action is string => action != null,
        ) ?? ["update_payment_method", "open_portal"],
        updatedAt: raw.lifecycle?.updatedAt ?? "",
      },
      tax: {
        configured: raw.tax?.configured ?? false,
        enabled: raw.tax?.enabled ?? false,
        reason: raw.tax?.reason ?? "",
        productTaxCode: raw.tax?.productTaxCode ?? "",
        taxBehavior: raw.tax?.taxBehavior ?? "",
        registrationCount: raw.tax?.registrationCount ?? 0,
      },
    };
  }, [data]);

  const openCheckout = useCallback(async () => {
    if (!currentWorkspaceId) return;
    // Preserve the click's user activation while the GraphQL request is in
    // flight. Browsers commonly block a tab first opened after the await.
    const checkoutWindow =
      checkoutTarget === "new-tab"
        ? window.open("about:blank", "_blank")
        : null;
    if (checkoutWindow) checkoutWindow.opener = null;
    setCheckoutBusy(true);
    try {
      const result = await createCheckout({
        variables: {
          workspaceId: currentWorkspaceId,
          successUrl: billingReturnURL("success"),
          cancelUrl: billingReturnURL("cancelled"),
        },
      });
      const url = result.data?.createBillingCheckoutSession?.url;
      if (!url) throw new Error("Checkout returned no hosted URL");
      if (checkoutTarget === "new-tab") {
        if (checkoutWindow) checkoutWindow.location.assign(url);
        else window.location.assign(url);
        setCheckoutBusy(false);
      } else {
        window.location.assign(url);
      }
    } catch {
      checkoutWindow?.close();
      toast.error(t("usage.billingCheckoutError"));
      setCheckoutBusy(false);
    }
  }, [checkoutTarget, createCheckout, currentWorkspaceId, t]);

  const openPortal = useCallback(async () => {
    if (!currentWorkspaceId) return;
    setPortalBusy(true);
    try {
      const result = await createPortal({
        variables: {
          workspaceId: currentWorkspaceId,
          returnUrl: billingReturnURL(),
        },
      });
      const url = result.data?.createBillingPortalSession?.url;
      if (!url) throw new Error("Portal returned no hosted URL");
      window.location.assign(url);
    } catch {
      toast.error(t("usage.billingPortalError"));
      setPortalBusy(false);
    }
  }, [createPortal, currentWorkspaceId, t]);

  return {
    readiness,
    loading: active && (!resolved || loading),
    error,
    checkoutBusy,
    portalBusy,
    openCheckout,
    openPortal,
    refetch,
  };
}
