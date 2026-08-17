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
import { Route as UsageShimRoute } from "../usage";
import { Route as BillingSplatRoute } from "../billing_.$";

/**
 * The w5/m70 rename made /billing the real money page and /usage a redirect
 * shim. These tests pin the redirect contract: bookmarks and pre-rename Stripe
 * return URLs (query included) keep working, and Render's sitewide
 * /billing/update-plan shape still opens the change-plan dialog.
 *
 * The /usage shim runs as a router integration; the /billing/$ splat's
 * beforeLoad is asserted directly (the render-alias.test.ts pattern) — a
 * hand-built route tree holding both an exact "/billing" and a "/billing/$"
 * splat ranks them insertion-order-sensitively, which made the integration
 * version flake in CI while the production file-based tree is unambiguous.
 */
function renderAt(initialPath: string) {
  const rootRoute = createRootRoute();
  const usageShim = createRoute({
    getParentRoute: () => rootRoute,
    path: "/usage",
    beforeLoad: UsageShimRoute.options.beforeLoad,
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
function splatRedirectHref(splat: string, href: string): string | undefined {
  const beforeLoad = BillingSplatRoute.options.beforeLoad as (ctx: {
    params: { _splat: string };
    location: ParsedLocation;
  }) => never;
  const pathname = href.split("?")[0]!;
  try {
    beforeLoad({
      params: { _splat: splat },
      location: { pathname, href } as ParsedLocation,
    });
  } catch (thrown) {
    return (thrown as { options?: { href?: string } }).options?.href;
  }
  return undefined;
}

describe("billing redirects (w5/m70)", () => {
  it("redirects /usage to /billing preserving the query string", async () => {
    const router = renderAt("/usage?billing=success");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/billing"),
    );
    expect(router.state.location.searchStr).toBe("?billing=success");
    // replace, not push — the shim must not stack a history entry.
    expect(router.history.length).toBe(1);
  });

  it("keeps /billing/update-plan opening the change-plan dialog", () => {
    expect(splatRedirectHref("update-plan", "/billing/update-plan")).toBe(
      "/workspace/settings?plan=change",
    );
  });

  it("folds any other /billing sub-path back to the billing page", () => {
    expect(splatRedirectHref("whatever/else", "/billing/whatever/else")).toBe(
      "/billing",
    );
    // Render appends its own query — it folds into the landing's.
    expect(splatRedirectHref("something", "/billing/something?next=x")).toBe(
      "/billing?next=x",
    );
  });
});
