import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
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

function renderPanel() {
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
  return render(<RouterProvider router={router} />);
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
