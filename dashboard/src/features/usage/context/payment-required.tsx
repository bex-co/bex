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

import { useCallback, useEffect, useRef, useState, useMemo } from "react";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { BillingOnboardingView } from "../components/billing-onboarding";
import { useBillingOnboarding } from "../hooks/use-billing-onboarding";
import {
  PaymentRequiredContext,
  type RetryAction,
} from "./payment-required-context";
import { PaymentOnboardingCancelledError } from "./payment-required-error";

type PendingAction = {
  workspaceId: string;
  action: RetryAction<unknown>;
  resolve: (value: unknown) => void;
  reject: (error: unknown) => void;
};

export function PaymentRequiredProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [open, setOpen] = useState(false);
  const [retrying, setRetrying] = useState(false);
  // Arms the fast (2s) readiness poll; cleared once a payment method is bound so
  // the dialog stops hammering bex-api after it's satisfied, re-armed on reopen
  // (w9/m62 t003). Driven by adjust-state-during-render below (not an effect).
  const [pollActive, setPollActive] = useState(true);
  const [prevOpen, setPrevOpen] = useState(open);
  const pendingRef = useRef<PendingAction | null>(null);
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const billing = useBillingOnboarding({
    active: open && pollActive,
    pollInterval: 2_000,
    checkoutTarget: "new-tab",
  });
  const paymentMethodReady = billing.readiness?.paymentMethodReady ?? false;
  const refetchBilling = billing.refetch;

  // Re-arm the poll when the dialog (re)opens, then stop it the moment a payment
  // method is bound — adjusted during render (guarded so it converges), the same
  // sanctioned prop-change pattern as use-deploy-logs' useWindowPolling, so no
  // effect calls setState (w9/m62 t003).
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (open) setPollActive(true);
  }
  if (pollActive && paymentMethodReady) {
    setPollActive(false);
  }

  const clearRetryTimer = useCallback(() => {
    if (retryTimerRef.current != null) {
      clearTimeout(retryTimerRef.current);
      retryTimerRef.current = null;
    }
  }, []);

  const cancelPending = useCallback(() => {
    clearRetryTimer();
    const pending = pendingRef.current;
    pendingRef.current = null;
    setRetrying(false);
    setOpen(false);
    pending?.reject(new PaymentOnboardingCancelledError());
  }, [clearRetryTimer]);

  const run = useCallback(
    async <T,>(action: RetryAction<T>): Promise<T> => {
      try {
        return await action();
      } catch (error) {
        if (!hasGraphQLErrorCode(error, "PAYMENT_REQUIRED")) throw error;
      }

      if (!currentWorkspaceId) {
        throw new Error("Payment onboarding requires a selected workspace");
      }
      if (pendingRef.current) cancelPending();
      return await new Promise<T>((resolve, reject) => {
        pendingRef.current = {
          workspaceId: currentWorkspaceId,
          action,
          resolve: resolve as (value: unknown) => void,
          reject,
        };
        setOpen(true);
      });
    },
    [cancelPending, currentWorkspaceId],
  );

  const retryPending = useCallback(async () => {
    const pending = pendingRef.current;
    if (!pending || retrying) return;
    setRetrying(true);
    try {
      const result = await pending.action();
      if (pendingRef.current !== pending) return;
      pendingRef.current = null;
      setOpen(false);
      pending.resolve(result);
    } catch (error) {
      if (pendingRef.current !== pending) return;
      if (!hasGraphQLErrorCode(error, "PAYMENT_REQUIRED")) {
        pendingRef.current = null;
        setOpen(false);
        setRetrying(false);
        pending.reject(error);
        return;
      }
      // Stripe readiness can become true milliseconds before the signed
      // webhook commits the local marker. Keep the interrupted action intact
      // and retry at the same bounded cadence until that commit is visible.
      retryTimerRef.current = setTimeout(() => {
        retryTimerRef.current = null;
        setRetrying(false);
        void refetchBilling();
      }, 2_000);
      return;
    }
    setRetrying(false);
  }, [refetchBilling, retrying]);

  useEffect(() => {
    if (open && paymentMethodReady && !retrying) {
      const timer = setTimeout(() => void retryPending(), 0);
      return () => clearTimeout(timer);
    }
  }, [open, paymentMethodReady, retryPending, retrying]);

  useEffect(() => {
    const refresh = () => {
      if (open) void refetchBilling();
    };
    window.addEventListener("focus", refresh);
    return () => window.removeEventListener("focus", refresh);
  }, [open, refetchBilling]);

  useEffect(() => {
    const pending = pendingRef.current;
    if (pending && pending.workspaceId !== currentWorkspaceId) {
      const timer = setTimeout(cancelPending, 0);
      return () => clearTimeout(timer);
    }
  }, [cancelPending, currentWorkspaceId]);

  useEffect(
    () => () => {
      clearRetryTimer();
      pendingRef.current?.reject(new PaymentOnboardingCancelledError());
      pendingRef.current = null;
    },
    [clearRetryTimer],
  );

  // A fresh object every render would re-render every consumer (and, since
  // w6/m42, invalidate the agent-session mutation identities) on each 2s
  // readiness poll while the dialog is open — memoize on the stable run.
  const gate = useMemo(() => ({ run }), [run]);

  return (
    <PaymentRequiredContext.Provider value={gate}>
      {children}
      <Dialog open={open} onOpenChange={(next) => !next && cancelPending()}>
        <DialogContent showCloseButton={false} className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("usage.paymentRequiredTitle")}</DialogTitle>
            <DialogDescription>
              {t("usage.paymentRequiredDescription")}
            </DialogDescription>
          </DialogHeader>
          <BillingOnboardingView
            readiness={billing.readiness}
            loading={billing.loading}
            error={billing.error}
            checkoutBusy={billing.checkoutBusy}
            portalBusy={billing.portalBusy}
            onCheckout={() => void billing.openCheckout()}
            onPortal={() => void billing.openPortal()}
          />
          {retrying && (
            <p className="text-sm text-muted-foreground" role="status">
              {t("usage.paymentRequiredRetrying")}
            </p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={cancelPending}>
              {t("usage.paymentRequiredCancel")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PaymentRequiredContext.Provider>
  );
}
