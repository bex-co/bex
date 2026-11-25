import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
  type RouteComponent,
} from "@tanstack/react-router";

// w3/099: every project page is a touch region, as a service's pages are.
// Below 64rem or on a coarse pointer, style.css grows each button, select and
// input inside a `data-touch-region` to a 44px target. Nothing in the project
// layout set one, so a project's Application Logs view kept the desktop's
// 32px Return to Bottom, Load older and range picker on a phone. Each loading
// frame of a project page reserves the grown boxes, so nothing moves when the
// page arrives.

const A = "tea-aaaaaaaaaaaaaaaaaaaa";
const B = "tea-bbbbbbbbbbbbbbbbbbbb";
const owner = vi.hoisted(() => ({ id: "" }));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    workspaces: [
      { id: A, name: "A" },
      { id: B, name: "B" },
    ],
    currentWorkspaceId: A,
    loading: false,
    setCurrentWorkspaceId: vi.fn(),
  }),
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

import { Route as ProjectRoute } from "../project.$projectId";
import {
  ProjectLogViewPageSkeleton,
  ProjectMetricsViewPageSkeleton,
  ProjectOverviewPageSkeleton,
  ProjectRouteSkeleton,
  ProjectSettingsPageSkeleton,
} from "@/common/components/route-skeletons";

async function renderLayout(component: RouteComponent) {
  const rootRoute = createRootRoute();
  const projectRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId",
    component,
    loader: () => ({
      state: "ready",
      resource: {
        id: "prj-1",
        name: "project-name",
        ownerId: owner.id,
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      },
    }),
  });
  const childRoute = createRoute({
    getParentRoute: () => projectRoute,
    path: "/",
    component: () => <button type="button">child page</button>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([projectRoute.addChildren([childRoute])]),
    history: createMemoryHistory({ initialEntries: ["/project/prj-1"] }),
  });
  const view = render(<RouterProvider router={router} />);
  await router.load();
  return view;
}

describe("the project layout's touch region (w3/099)", () => {
  it("holds the page's controls", async () => {
    owner.id = A;
    await renderLayout(ProjectRoute.options.component as RouteComponent);
    const control = await screen.findByRole("button", { name: "child page" });
    expect(control.closest("[data-touch-region]")).not.toBeNull();
  });

  it("holds the skeleton it shows while the owning workspace is adopted", async () => {
    owner.id = B;
    const { container } = await renderLayout(
      ProjectRoute.options.component as RouteComponent,
    );
    const frame = await vi.waitFor(() => {
      const found = container.querySelector(
        '[data-route-skeleton="project-active-child"]',
      );
      expect(found).not.toBeNull();
      return found!;
    });
    expect(frame.closest("[data-touch-region]")).not.toBeNull();
    expect(screen.queryByText("child page")).toBeNull();
  });

  it("is the route's pending frame too", async () => {
    expect(ProjectRoute.options.pendingComponent).toBe(ProjectRouteSkeleton);
    const { container } = await renderLayout(ProjectRouteSkeleton);
    const frame = await vi.waitFor(() => {
      const found = container.querySelector(
        '[data-route-skeleton="project-active-child"]',
      );
      expect(found).not.toBeNull();
      return found!;
    });
    expect(frame.querySelector("[data-touch-region]")).not.toBeNull();
  });
});

// A box standing in for a control: a skeleton bar or a reserved button at a
// control's height. A text bar (h-3 to h-6) keeps its size.
const CONTROL_HEIGHT = /^(h-(7|8|9|10)|size-(8|9))$/;

function controlBoxes(root: HTMLElement): HTMLElement[] {
  return [
    ...root.querySelectorAll<HTMLElement>(
      '[data-slot="skeleton"], span.invisible',
    ),
  ].filter((el) => [...el.classList].some((name) => CONTROL_HEIGHT.test(name)));
}

describe.each([
  ["Overview", ProjectOverviewPageSkeleton],
  ["Settings", ProjectSettingsPageSkeleton],
  ["log view", ProjectLogViewPageSkeleton],
  ["Service Metrics view", ProjectMetricsViewPageSkeleton],
])("the project %s skeleton (w3/099)", (_, Skeleton) => {
  it("reserves every control at the touch region's 44px", () => {
    const { container } = render(<Skeleton />);
    const boxes = controlBoxes(container);
    expect(boxes.length).toBeGreaterThan(0);
    for (const box of boxes) {
      const square = [...box.classList].some((name) => /^size-/.test(name));
      const touch = square
        ? ["max-lg:size-11", "pointer-coarse:size-11"]
        : ["max-lg:h-11", "pointer-coarse:h-11"];
      expect(box, box.className).toHaveClass(...touch);
    }
  });
});
