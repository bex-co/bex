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
  tax: BillingTaxReadiness;
}

export interface UseBillingOnboardingResult {
  readiness: BillingReadiness | null;
  loading: boolean;
  error: Error | undefined;
  checkoutBusy: boolean;
  portalBusy: boolean;
  openCheckout: () => Promise<void>;
  openPortal: () => Promise<void>;
}

function billingReturnURL(state?: "success" | "cancelled"): string {
  const url = new URL("/usage", window.location.origin);
  if (state) url.searchParams.set("billing", state);
  return url.toString();
}

export function useBillingOnboarding(): UseBillingOnboardingResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error } = useQuery(BillingReadinessDocument, {
    variables: { workspaceId: currentWorkspaceId ?? "" },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    pollInterval: 15_000,
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
      window.location.assign(url);
    } catch {
      toast.error(t("usage.billingCheckoutError"));
      setCheckoutBusy(false);
    }
  }, [createCheckout, currentWorkspaceId, t]);

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
    loading: !resolved || loading,
    error,
    checkoutBusy,
    portalBusy,
    openCheckout,
    openPortal,
  };
}
