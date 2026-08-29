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
import { useQuery } from "@apollo/client/react";
import { useNavigate } from "@tanstack/react-router";
import { BillingReadinessDocument } from "@/graphql/definitions";
import { currentHref } from "@/common/lib/safe-next";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { paymentSetupGateBlocks, paymentSetupPath } from "../lib/payment-setup";

/**
 * The payment wall's backstop for every chrome (authenticated, in-app) route
 * (ADR075 D7, revised 2026-08-29): a workspace the `all` gate still refuses
 * cannot use the product, so its billing manager is sent to `/setup/payment`
 * with the current href as the guarded `next`. The sign-up flow lands on the
 * wall directly (verification success → wall), so this only catches the user
 * who left the wall and came back — a later sign-in, another device, a
 * bookmarked deep link.
 *
 * Fail-open by construction: the redirect fires only on the server's
 * definitive `paymentMethodOnboardingRequired: true`. A loading, errored
 * (a non-manager's 403), or forbidden read renders the page as usual — the
 * API's 402 + interception dialog remain the gate for those callers.
 *
 * Reads the same query + variables as the wall and the billing page, so one
 * normalized cache entry serves all three; `cache-first` means a bound
 * workspace costs nothing after its first read.
 */
export function PaymentSetupGate({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const { currentWorkspaceId } = useWorkspace();
  const capabilities = useCapabilities();
  const billingForbidden =
    capabilities.loaded && !capabilities.canManageBilling;
  const { data } = useQuery(BillingReadinessDocument, {
    variables: { workspaceId: currentWorkspaceId ?? "" },
    skip: currentWorkspaceId == null || billingForbidden,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  const blocked = paymentSetupGateBlocks({
    onboardingRequired:
      data?.workspaceBillingReadiness?.paymentMethodOnboardingRequired,
    billingForbidden,
  });

  useEffect(() => {
    if (!blocked) return;
    void navigate({
      to: "/",
      href: paymentSetupPath(currentHref()),
      replace: true,
    });
  }, [blocked, navigate]);

  // Paint nothing while the redirect is in flight: the page underneath would
  // otherwise fire its own queries and flash for one frame.
  if (blocked) return null;
  return children;
}
