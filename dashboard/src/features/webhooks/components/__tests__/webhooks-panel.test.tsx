import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { serverRendered } from "@/test/hydration";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { WebhooksPanel } from "@/features/webhooks/components/webhooks-panel";
import type { WebhookEndpointView } from "@/features/webhooks/types";

const setEnabled = vi.fn();
let canManage = true;
let endpoints: WebhookEndpointView[] = [];

vi.mock("@/features/webhooks/hooks/use-webhooks", () => ({
  useWebhooks: () => ({
    endpoints,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/webhooks/hooks/use-set-webhook-enabled", () => ({
  useSetWebhookEnabled: () => ({ setEnabled, toggling: null }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({ canManage, loaded: true }),
}));

function endpoint(
  id: string,
  name: string,
  overrides: Partial<WebhookEndpointView> = {},
): WebhookEndpointView {
  return {
    id,
    name,
    url: `https://${name}.example.test/hook`,
    eventTypes: [
      "deploy_started",
      "deploy_ended",
      "build_started",
      "maintenance_mode_enabled",
    ],
    enabled: true,
    disabledReason: "",
    createdAt: "2026-08-17T00:00:00Z",
    createdBy: "",
    latestStatus: "",
    latestSentAt: null,
    latestParentStatus: "",
    ...overrides,
  };
}

function panelRouter() {
  const root = createRootRoute();
  const index = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: WebhooksPanel,
  });
  const detail = createRoute({
    getParentRoute: () => root,
    path: "/webhook/$webhookId",
    component: () => <div />,
  });
  const create = createRoute({
    getParentRoute: () => root,
    path: "/webhooks/new",
    component: () => <div />,
  });
  const router = createRouter({
    routeTree: root.addChildren([index, detail, create]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return router;
}

function renderPanel() {
  return render(<RouterProvider router={panelRouter()} />);
}

beforeEach(() => {
  canManage = true;
  endpoints = [
    endpoint("whk-success", "payments", {
      latestStatus: "delivered",
      latestParentStatus: "delivered",
      latestSentAt: "2026-08-17T12:00:00Z",
    }),
    endpoint("whk-retry", "deploy-alerts", {
      eventTypes: ["service_suspended"],
      latestStatus: "failed",
      latestParentStatus: "pending",
      latestSentAt: "2026-08-17T11:00:00Z",
    }),
    endpoint("whk-failed", "pager", {
      eventTypes: ["build_ended"],
      latestStatus: "failed",
      latestParentStatus: "failed",
      latestSentAt: "2026-08-17T10:00:00Z",
    }),
    endpoint("whk-never", "unused", { eventTypes: [] }),
  ];
  setEnabled.mockReset();
});

describe("WebhooksPanel", () => {
  it("renders latest immutable outcomes and bounds event chips", async () => {
    const user = userEvent.setup();
    renderPanel();

    expect(await screen.findByText("Successful")).toBeInTheDocument();
    expect(screen.getByText("Retrying")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
    expect(screen.getByText("Never sent")).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "Show 1 more" }).length,
    ).toBeGreaterThan(0);

    const payments = screen.getByText("payments").closest("tr")!;
    expect(payments).not.toHaveTextContent("Maintenance Mode Enabled");
    await user.click(screen.getAllByRole("button", { name: "Show 1 more" })[0]);
    expect(payments).toHaveTextContent("Maintenance Mode Enabled");
  });

  it("searches names, destinations, and translated event labels", async () => {
    const user = userEvent.setup();
    renderPanel();
    const search = await screen.findByRole("searchbox", {
      name: "Search webhooks",
    });

    await user.type(search, "Suspended");
    expect(screen.getByText("deploy-alerts")).toBeInTheDocument();
    expect(screen.queryByText("payments")).not.toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "pager.example.test");
    expect(screen.getByText("pager")).toBeInTheDocument();
    expect(screen.queryByText("unused")).not.toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "no-such-hook");
    expect(
      screen.getByText("No webhooks match your search."),
    ).toBeInTheDocument();
  });

  it("keeps member reads but removes every active management affordance", async () => {
    canManage = false;
    renderPanel();
    expect(await screen.findByText("payments")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Add webhook" }),
    ).not.toBeInTheDocument();
    expect(screen.queryAllByRole("switch")).toHaveLength(0);
    expect(screen.getAllByText("Enabled").length).toBeGreaterThan(0);
  });
});

// w4/m220 (a832): the list's loading preview must be the ready card itself,
// or the header and table jump on load. Pixel parity is a browser measurement;
// this pins the shared copy, action and columns.
it("previews the ready card's copy, action, search help and columns (w4/m220)", async () => {
  const ready = renderPanel();
  await screen.findByText("payments");
  const text = (el: Element | null) => el?.textContent?.trim();
  const readyAction = text(
    ready.container.querySelector('[data-slot="card-action"]'),
  );
  const readyTitle = text(
    ready.container.querySelector('[data-slot="card-title"]'),
  );
  const readyLabels = [...ready.container.querySelectorAll("th")].map(text);
  ready.unmount();

  const { WebhooksListPageSkeleton } =
    await import("@/common/components/route-skeletons");
  const { container } = render(<WebhooksListPageSkeleton />);
  expect(readyAction).toBeTruthy();
  expect(text(container.querySelector('[data-slot="card-action"]'))).toBe(
    readyAction,
  );
  expect(text(container.querySelector('[data-slot="card-title"]'))).toBe(
    readyTitle,
  );
  expect([...container.querySelectorAll("th")].map(text)).toEqual(readyLabels);
  // Every reserved bar sits in a positioned box, never a <p> it would escape.
  expect(container.querySelector("p [data-slot='skeleton']")).toBeNull();
});

// w4/m219 (a837): a query typed into the server HTML before handlers attach
// is wiped by hydration.
it("keeps the search uneditable until hydration, then filters (w4/m219)", async () => {
  const router = panelRouter();
  await router.load();
  const { container, hydrate, getByRole } = serverRendered(
    <RouterProvider router={router} />,
  );
  const search = () =>
    getByRole<HTMLInputElement>("searchbox", { name: "Search webhooks" });
  expect(search().disabled).toBe(true);
  await hydrate();
  expect(search().disabled).toBe(false);
  await userEvent.type(search(), "pager");
  expect(search().value).toBe("pager");
  expect(container).toHaveTextContent("pager");
  expect(container).not.toHaveTextContent("payments");
});

// w4/410: the page's visible title is its one h1.
describe("WebhooksPanel heading", () => {
  it("renders the title as the page's h1", async () => {
    renderPanel();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Webhooks" }),
    ).toBeInTheDocument();
  });
});
