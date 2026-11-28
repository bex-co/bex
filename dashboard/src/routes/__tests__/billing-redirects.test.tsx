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

import { describe, it, expect } from "vitest";
import { render, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createRootRoute,
  createRoute,
  createMemoryHistory,
  createRouter,
  type ParsedLocation,
} from "@tanstack/react-router";
import { routeTree } from "@/routeTree.gen";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";
import { Route as BillingSplatRoute } from "../billing_.$first.$";
import { Route as PlanConfirmRoute } from "../billing_.update-plan-confirm";
import { Route as PlanPreviewRoute } from "../billing_.update-plan-preview";

/**
 * The w5/m70 rename made /billing the real money page and /usage a redirect
 * shim. These tests pin the redirect contract: bookmarks and pre-rename Stripe
 * return URLs (query included) keep working, and Render's plan pages
 * (/billing/update-plan and its confirmation and preview, w3/m708) answer on
 * routes of their own.
 *
 * The /usage shim runs as a router integration; the constrained
 * /billing/$first/$ splat's beforeLoad is asserted directly (the
 * render-alias.test.ts pattern). The generated production tree is also checked
 * because an unconstrained /billing/$ splat matches the empty suffix and
 * outranks the exact page during SSR, producing a /billing -> /billing loop.
 */
function renderAt(initialPath: string) {
  const rootRoute = createRootRoute();
  // Mirrors ../usage's shim beforeLoad exactly (same production helper): a
  // file-route's beforeLoad type is bound to the real registered root and can't
  // be reattached to a synthetic root, so delegate to the shared helper here.
  const usageShim = createRoute({
    getParentRoute: () => rootRoute,
    path: "/usage",
    beforeLoad: ({ location }) => {
      redirectPreservingSuffix("/billing", location);
    },
  });
  const billing = createRoute({
    getParentRoute: () => rootRoute,
    path: "/billing",
    component: () => <div>landed:/billing</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([usageShim, billing]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

/** beforeLoad throws a redirect; capture its href. */
function splatRedirectHref(first: string, href: string): string | undefined {
  const beforeLoad = BillingSplatRoute.options.beforeLoad as (ctx: {
    params: { first: string; _splat: string };
    location: ParsedLocation;
  }) => never;
  const pathname = href.split("?")[0]!;
  try {
    beforeLoad({
      params: { first, _splat: "" },
      location: { pathname, href } as ParsedLocation,
    });
  } catch (thrown) {
    return (thrown as { options?: { href?: string } }).options?.href;
  }
  return undefined;
}

describe("billing redirects (w5/m70)", () => {
  it("matches bare /billing to the real page, never the redirect alias", () => {
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ["/billing"] }),
      context: { client: {} as never },
    });

    expect(
      router.matchRoutes("/billing").map((match) => match.routeId),
    ).toEqual(["__root__", "/billing"]);
  });

  it("redirects /usage to /billing preserving the query string", async () => {
    const router = renderAt("/usage?billing=success");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/billing"),
    );
    expect(router.state.location.searchStr).toBe("?billing=success");
    // replace, not push — the shim must not stack a history entry.
    expect(router.history.length).toBe(1);
  });

  // w3/m708: Render's plan pages are routes of their own, which outrank the
  // splat that folds every other /billing sub-path back to Billing.
  it.each([
    ["/billing/update-plan", "/billing_/update-plan"],
    ["/billing/update-plan-confirm", "/billing_/update-plan-confirm"],
    ["/billing/update-plan-preview", "/billing_/update-plan-preview"],
  ])("matches %s to its own route, never the splat", (path, routeId) => {
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: [path] }),
      context: { client: {} as never },
    });

    expect(router.matchRoutes(path).map((match) => match.routeId)).toEqual([
      "__root__",
      routeId,
    ]);
  });

  it("folds any other /billing sub-path back to the billing page", () => {
    expect(splatRedirectHref("whatever", "/billing/whatever/else")).toBe(
      "/billing",
    );
    // Render appends its own query — it folds into the landing's.
    expect(splatRedirectHref("something", "/billing/something?next=x")).toBe(
      "/billing?next=x",
    );
  });

  // w3/m708: Render opens its preview only for a hard custom-domain limit;
  // bex meters custom domains, so the preview goes on to the confirmation.
  it("sends the preview on to the confirmation with the same plan", () => {
    expect(
      thrownRedirect(PlanPreviewRoute, {
        search: { plan: "hobby", next: "/webhooks" },
      }),
    ).toMatchObject({
      to: "/billing/update-plan-confirm",
      search: { plan: "hobby", next: "/webhooks" },
      replace: true,
    });
    expect(
      thrownRedirect(PlanPreviewRoute, { search: { next: "/webhooks" } }),
    ).toMatchObject({
      to: "/billing/update-plan",
      search: { next: "/webhooks" },
    });
  });

  it("sends a confirmation with no plan back to the plan page, as Render's does", () => {
    expect(
      thrownRedirect(PlanConfirmRoute, {
        search: { next: "/webhooks" },
        context: { session: { id: "s" } },
        location: { href: "/billing/update-plan-confirm" },
      }),
    ).toMatchObject({
      to: "/billing/update-plan",
      search: { next: "/webhooks" },
    });
    expect(
      thrownRedirect(PlanConfirmRoute, {
        search: { plan: "hobby" },
        context: { session: { id: "s" } },
        location: { href: "/billing/update-plan-confirm?plan=hobby" },
      }),
    ).toBeUndefined();
  });
});

/** The redirect a route's beforeLoad throws for these arguments, if any. */
function thrownRedirect(
  route: { options: { beforeLoad?: unknown } },
  args: Record<string, unknown>,
): Record<string, unknown> | undefined {
  try {
    (route.options.beforeLoad as (a: unknown) => void)(args);
  } catch (thrown) {
    return (thrown as { options: Record<string, unknown> }).options;
  }
  return undefined;
}
