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
import { useNavigate } from "@tanstack/react-router";
import { currentHref } from "@/common/lib/safe-next";
import { useRootContext } from "@/common/hooks/use-root-context";
import { VerificationRouteSkeleton } from "@/common/components/route-skeletons";
import {
  emailVerificationPath,
  emailVerificationRequired,
} from "../lib/email-verification";

/**
 * The verification wall for every chrome (authenticated, in-app) route
 * (ADR075 D8 revision 2026-10-06, w2/m168): a human whose trait email is not
 * verified can't use bex, so a signed-in unverified session is sent to
 * `/auth/verification` with the current href as the guarded `next`. Success
 * there continues to `/setup/payment` (D7) and then to `next`.
 *
 * Runs BEFORE `PaymentSetupGate` and outside the dashboard shell: the shell
 * itself (workspace switcher, sidebar) is app content that reads bex-api,
 * which refuses this caller anyway. Bare routes (auth/*, consent/device,
 * /invite, /setup/*, 404) mount neither gate, so they stay reachable.
 *
 * The source of truth is the Kratos session the root route already fetched
 * (no extra request). Fail-open on anything indeterminate (see
 * `emailVerificationRequired`); bex-api's `EMAIL_VERIFICATION_REQUIRED` 403,
 * caught by the Apollo auth link, is the backstop. A definitive unverified
 * session never renders app content: while the redirect is in flight the gate
 * paints the verification route's own pending skeleton, so the swap onto the
 * wall does not shift.
 */
export function EmailVerificationGate({
  children,
}: {
  children: React.ReactNode;
}) {
  const navigate = useNavigate();
  const { session } = useRootContext();
  const blocked = emailVerificationRequired(session);

  useEffect(() => {
    if (!blocked) return;
    void navigate({
      to: "/",
      href: emailVerificationPath(currentHref()),
      replace: true,
    });
  }, [blocked, navigate]);

  if (blocked) return <VerificationRouteSkeleton />;
  return children;
}
