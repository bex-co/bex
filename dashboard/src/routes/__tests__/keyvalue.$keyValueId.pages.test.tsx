import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { screen, waitFor } from "@testing-library/react";
import { Route as IndexRoute } from "../keyvalue.$keyValueId.index";
import { Route as InfoRoute } from "../keyvalue.$keyValueId.info";
import { Route as LogsRoute } from "../keyvalue.$keyValueId.logs";
import { Route as MetricsRoute } from "../keyvalue.$keyValueId.metrics";
import { renderKeyValuePages } from "@/test/datastore-pages";

// w3/m649: a Key Value instance's one Overview, its section rail and its
// Overview/Metrics/Logs tab bar became Render's pages, each at its own URL
// under one layout. Each page opens with its heading and holds exactly the
// cards its Render counterpart holds, in Render's order
// (docs/render-artifacts/key-value.md § Page map), and no other page's. The
// real layout and page routes run; the data hooks are stubbed and every card
// stands in as a named marker.

const { card } = vi.hoisted(() => ({
  card: (name: string) => () => <div data-card={name} />,
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));
vi.mock("@/features/workspaces/hooks/use-adopt-resource-workspace", () => ({
  useAdoptResourceWorkspace: () => false,
}));
vi.mock("@/features/keyvalue/hooks/use-key-value", () => ({
  useKeyValue: (id: string) => ({
    keyValue: {
      id,
      name: "sessions-cache",
      ownerId: "tea-1",
      status: "available",
      plan: "starter",
      version: "8",
      createdAt: null,
      externalHost: null,
      public: false,
      suspended: false,
      region: null,
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => ({ instanceTypes: [] }),
}));
vi.mock("@/features/keyvalue/components/key-value-status-badge", () => ({
  KeyValueStatusBadge: () => null,
}));
vi.mock("@/features/keyvalue/components/key-value-metadata-card", () => ({
  KeyValueMetadataCard: card("details"),
}));
vi.mock(
  "@/features/keyvalue/components/key-value-maxmemory-policy-section",
  () => ({ KeyValueMaxmemoryPolicySection: card("maxmemory-policy") }),
);
vi.mock(
  "@/features/keyvalue/components/key-value-persistence-mode-section",
  () => ({ KeyValuePersistenceModeSection: card("persistence-mode") }),
);
vi.mock("@/features/keyvalue/components/key-value-plan-section", () => ({
  KeyValuePlanSection: card("plan"),
}));
vi.mock("@/features/keyvalue/components/connection-info-panel", () => ({
  ConnectionInfoPanel: card("connections"),
}));
vi.mock("@/features/keyvalue/components/key-value-networking-panel", () => ({
  KeyValueNetworkingPanel: card("networking"),
}));
vi.mock("@/features/keyvalue/components/key-value-danger-actions", () => ({
  KeyValueDangerActions: card("danger-zone"),
}));
vi.mock("@/features/logs/components/datastore-log-viewer", () => ({
  DatastoreLogViewer: card("logs"),
}));
vi.mock("@/features/metrics/components/datastore-metrics-panel", () => ({
  DatastoreMetricsPanel: card("metrics"),
}));

const ROUTES = {
  "/": IndexRoute,
  "/info": InfoRoute,
  "/logs": LogsRoute,
  "/metrics": MetricsRoute,
};

// Render's Info: General (Maxmemory Policy and Persistence Mode are its
// fields), Compute, Connections, Networking, the delete card.
const INFO = [
  "details",
  "maxmemory-policy",
  "persistence-mode",
  "plan",
  "connections",
  "networking",
  "danger-zone",
];

const cards = () =>
  [...document.querySelectorAll<HTMLElement>("[data-card]")].map(
    (element) => element.dataset.card,
  );

beforeEach(() => {
  document.body.innerHTML = "";
});

describe("a Key Value instance's pages (w3/m649)", () => {
  it.each([
    ["/keyvalue/red-main", "Info", INFO],
    ["/keyvalue/red-main/info", "Info", INFO],
    ["/keyvalue/red-main/logs", "Logs", ["logs"]],
    ["/keyvalue/red-main/metrics", "Metrics", ["metrics"]],
  ])("%s opens %s with only its cards", async (href, heading, expected) => {
    await renderKeyValuePages(ROUTES, href);

    expect(
      await screen.findByRole("heading", { level: 1, name: "sessions-cache" }),
    ).toBeVisible();
    expect(
      await screen.findByRole("heading", { level: 2, name: heading }),
    ).toBeVisible();
    await waitFor(() => expect(cards()).toEqual(expected));
    // The old tab bar and section rail are gone.
    expect(screen.queryByRole("button", { name: "Overview" })).toBeNull();
    expect(screen.queryByRole("navigation")).toBeNull();
  });

  it.each([
    ["metadata", "metadata"],
    ["networking", "networking"],
    ["danger-zone", "danger-zone"],
    ["plan", "plan"],
    // Render's Upgrade instance link names the plan card #compute.
    ["compute", "plan"],
  ])("scrolls an Info link's #%s to its card", async (anchor, card) => {
    const scrolled: string[] = [];
    const spy = vi
      .spyOn(Element.prototype, "scrollIntoView")
      .mockImplementation(function (this: Element) {
        scrolled.push(this.id);
      });
    try {
      const router = await renderKeyValuePages(
        ROUTES,
        `/keyvalue/red-main/info#${anchor}`,
      );
      // The router scrolls to a hash it can find as it navigates; the
      // layout scrolls again once the instance's cards render.
      await waitFor(() => expect(scrolled.at(-1)).toBe(card));
      expect(new Set(scrolled)).toEqual(new Set([card]));
      expect(router.state.location.pathname).toBe("/keyvalue/red-main/info");
      expect(router.state.location.hash).toBe(anchor);
    } finally {
      spy.mockRestore();
    }
  });
});
