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

import { beforeEach, describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { BillingPageSkeleton } from "@/common/components/route-skeletons";
import { PlanCard } from "@/features/usage/components/plan-card";
import { PaymentMethodCardView } from "@/features/usage/components/payment-method-card";
import { ResourceCaps } from "@/features/usage/components/resource-caps";
import { FreeHoursCard } from "@/features/usage/components/free-hours-card";
import { BuildMinutesCard } from "@/features/usage/components/build-minutes-card";
import { IncludedBandwidthCard } from "@/features/usage/components/included-bandwidth-card";
import { CustomDomainsCard } from "@/features/usage/components/custom-domains-card";
import { CreditBalanceCard } from "@/features/usage/components/billing-summary-cards";
import type { BillingReadiness } from "@/features/usage/hooks/use-billing-onboarding";
import type { CustomDomainsUsage } from "@/features/usage/hooks/use-usage";
import type { WorkspaceView } from "@/features/workspaces/types";

// The Billing page's pending frame has to preview each ready card of the
// default view, the current month, with the same structure (w4/707).
const workspaceState = vi.hoisted(() => ({
  currentWorkspace: null as WorkspaceView | null,
  workspaces: [] as WorkspaceView[],
  loading: false,
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => workspaceState,
}));

const SCALE: WorkspaceView = {
  id: "tea-scale",
  name: "acme",
  slug: "acme",
  plan: "scale",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: null,
  primary: false,
  privacy: "invite_only",
};

const ORG_PRIMARY: WorkspaceView = {
  ...SCALE,
  id: "tea-primary",
  organization: {
    id: "org-1",
    name: "Acme",
    primaryWorkspaceId: "tea-primary",
    requireTwoFactor: false,
    requireGoogleLogin: false,
  },
  primary: true,
};
const ORG_SECONDARY: WorkspaceView = {
  ...ORG_PRIMARY,
  id: "tea-secondary",
  primary: false,
};

// A healthy workspace with a card on file: no badge, no banner.
const READINESS: BillingReadiness = {
  workspaceId: "tea-scale",
  mode: "live",
  customerReady: true,
  subscriptionReady: true,
  paymentMethodReady: true,
  paymentMethodBrand: "visa",
  paymentMethodLast4: "4242",
  paymentMethodRequired: false,
  paymentMethodOnboardingRequired: false,
  paymentMethodRejection: "",
  lifecycle: {
    status: "healthy",
    reason: "",
    graceDeadline: "",
    enforcementOwned: false,
    recoveryPending: false,
    allowedActions: [],
    updatedAt: "",
  },
  tax: {
    configured: true,
    enabled: true,
    reason: "",
    productTaxCode: "",
    taxBehavior: "",
    registrationCount: 0,
  },
};

// bex-api's caps on every plan (QuotaCapsForPlan), with the one-free-instance
// rule on both datastores (w4/m241).
const LIMITS = {
  services: { used: 13, terminating: 0, limit: 100, freeUsed: 0, freeLimit: 0 },
  postgres: { used: 4, terminating: 0, limit: 25, freeUsed: 0, freeLimit: 1 },
  keyValues: { used: 2, terminating: 0, limit: 25, freeUsed: 0, freeLimit: 1 },
};

function ReadyBilling({ domains }: { domains: CustomDomainsUsage }) {
  return (
    <>
      <section id="plan">
        <PlanCard />
      </section>
      <section id="payment-method">
        <PaymentMethodCardView
          readiness={READINESS}
          loading={false}
          checkoutBusy={false}
          portalBusy={false}
          onCheckout={() => undefined}
          onPortal={() => undefined}
          canManageBilling
        />
      </section>
      <section id="included-usage">
        <ResourceCaps limits={LIMITS} />
        <FreeHoursCard hours={{ used: 28.8, included: 750 }} />
        <BuildMinutesCard minutes={{ used: 138, included: 5000 }} />
        <IncludedBandwidthCard bandwidth={{ used: 283.44, included: 1000 }} />
        <CustomDomainsCard domains={domains} />
      </section>
      <section id="credit-balance">
        <CreditBalanceCard
          billing={{
            currentCost: {
              amountUsd: "10.99",
              creditsAppliedUsd: "10.99",
              amountDueUsd: "0.00",
              currency: "USD",
              periodStart: "2026-11-01T00:00:00Z",
              periodEnd: "2026-12-01T00:00:00Z",
            },
            invoices: [],
            credits: {
              availableUsd: "1000.00",
              currency: "USD",
              grants: [
                { name: "grant", remainingUsd: "1000.00", expiresAt: "" },
              ],
            },
          }}
        />
      </section>
    </>
  );
}

async function renderReady(domains: CustomDomainsUsage) {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/billing",
    component: () => <ReadyBilling domains={domains} />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/billing"] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

const text = (element: Element) => element.textContent?.trim() ?? "";
const classes = (element: Element) =>
  [...element.classList].filter((name) => name !== "invisible").sort();

/**
 * A card stack's geometry-bearing frames: every card's title and description
 * copy, each button's label and box classes, the plan box, the payment card
 * line, cap tiles, used lines, meters, and the small `text-xs` lines. A
 * skeleton bar standing in for an element counts as it: a reserved button is
 * the invisible box `ReservedButton` lays out.
 */
function frames(root: Element) {
  const all = (selector: string) => [...root.querySelectorAll(selector)];
  const count = (selector: string) => all(selector).length;
  return {
    cards: count("[data-slot='card']"),
    titles: all("[data-slot='card-title']").map(text),
    descriptions: all("[data-slot='card-description']").map(text),
    buttons: [
      ...all("a[data-slot='button'], button"),
      ...all("span.relative.inline-flex > span.invisible"),
    ].map((button) => [text(button), classes(button)]),
    planBoxes: count(".rounded-md.border.p-4"),
    planNotes: count(
      "[data-testid='organization-plan-note'], [data-skeleton-region='organization-plan-note']",
    ),
    cardLines: count(".flex.items-center.gap-3.text-sm"),
    capTiles: count(".overflow-hidden.rounded-lg.border"),
    usedLines: count("p.text-sm.font-medium"),
    meters: count(".h-2.w-full.rounded-full"),
    smallLines: count("p.text-xs"),
  };
}

function skeletonRegion(name: string) {
  const { container } = render(<BillingPageSkeleton />);
  const element = container.querySelector(`[data-skeleton-region="${name}"]`);
  expect(element).not.toBeNull();
  return element!;
}

const SCALE_DOMAINS = { used: 7, included: 25, unlimited: false };

beforeEach(() => {
  workspaceState.currentWorkspace = SCALE;
  workspaceState.workspaces = [SCALE];
});

describe("BillingPageSkeleton (w4/707)", () => {
  it.each(["plan", "payment-method", "included-usage", "credit-balance"])(
    "previews the ready %s section's frames",
    async (name) => {
      const ready = await renderReady(SCALE_DOMAINS);
      const section = ready.container.querySelector(`section#${name}`)!;
      expect(frames(section).cards).toBeGreaterThan(0);
      expect(frames(skeletonRegion(name))).toEqual(frames(section));
      ready.unmount();
    },
  );

  it("reserves all five included-usage cards outside Enterprise", () => {
    const included = skeletonRegion("included-usage");
    expect(frames(included)).toMatchObject({ cards: 5, capTiles: 3 });
  });

  it("drops the custom domains card on Enterprise, as the ready page does", async () => {
    const enterprise = { ...SCALE, plan: "enterprise" };
    workspaceState.currentWorkspace = enterprise;
    workspaceState.workspaces = [enterprise];
    const ready = await renderReady({ used: 7, included: 0, unlimited: true });
    const section = ready.container.querySelector("section#included-usage")!;
    const pending = frames(skeletonRegion("included-usage"));
    expect(pending.cards).toBe(4);
    expect(pending).toEqual(frames(section));
    ready.unmount();
  });

  it.each([
    ["a primary", ORG_PRIMARY],
    ["a secondary", ORG_SECONDARY],
  ])(
    "draws %s workspace's plan card as the ready card does",
    async (_, workspace) => {
      workspaceState.currentWorkspace = workspace;
      workspaceState.workspaces = [ORG_PRIMARY, ORG_SECONDARY];
      const ready = await renderReady(SCALE_DOMAINS);
      const section = ready.container.querySelector("section#plan")!;
      expect(frames(skeletonRegion("plan"))).toEqual(frames(section));
      ready.unmount();
    },
  );
});
