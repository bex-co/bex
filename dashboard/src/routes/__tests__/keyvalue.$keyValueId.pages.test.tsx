import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route as IndexRoute } from "../keyvalue.$keyValueId.index";
import { Route as InfoRoute } from "../keyvalue.$keyValueId.info";
import { Route as LogsRoute } from "../keyvalue.$keyValueId.logs";
import { Route as MetricsRoute } from "../keyvalue.$keyValueId.metrics";
import {
  datastoreHeaderGeometry as headerGeometry,
  renderDatastorePendingFrame,
  renderKeyValuePages,
} from "@/test/datastore-pages";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

// w3/m649: a Key Value instance's one Overview, its section rail and its
// Overview/Metrics/Logs tab bar became Render's pages, each at its own URL
// under one layout. Each page opens with its heading and holds exactly the
// cards its Render counterpart holds, in Render's order
// (docs/render-artifacts/key-value.md § Page map), and no other page's. The
// real layout and page routes run; the data hooks are stubbed and every card
// stands in as a named marker.

const { card, kv, catalog } = vi.hoisted(() => ({
  card: (name: string) => () => <div data-card={name} />,
  // Whether an IP rule admits external traffic (w3/m675).
  kv: { public: false },
  catalog: { current: [] as { id: string; name: string }[] },
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
      public: kv.public,
      suspended: false,
      region: null,
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => ({ instanceTypes: catalog.current }),
}));
// The header's Connect reads with the Info card's own fetch (w3/m675).
// One client for the whole file, as the provider's is.
const apolloClient = vi.hoisted(() => ({}));
vi.mock("@apollo/client/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@apollo/client/react")>()),
  useApolloClient: () => apolloClient,
}));
vi.mock("@/features/keyvalue/hooks/use-connection-info", () => ({
  fetchConnectionInfo: async () => ({
    internalConnectionString: "redis://red-main:6379",
    externalConnectionString: "rediss://red-main:pw@red-main.kv.bex.co:6379",
    cliCommand: "valkey-cli --tls",
  }),
}));
vi.mock("@/features/keyvalue/components/key-value-status-badge", () => ({
  // A stand-in badge, so the header keeps its ready items.
  KeyValueStatusBadge: () => <span>Available</span>,
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
  kv.public = false;
  catalog.current = [];
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

// w3/m675: every page's header is Render's: the name, the status, the plan
// badge opening Info's plan card (Render's `/info#compute`), View docs and
// Connect (docs/render-artifacts/key-value.md § Header).
describe("a Key Value instance's header (w3/m675)", () => {
  it.each(["", "/info", "/logs", "/metrics"])(
    "/keyvalue/red-main%s carries Render's header",
    async (page) => {
      catalog.current = [{ id: "starter", name: "Starter" }];
      await renderKeyValuePages(ROUTES, `/keyvalue/red-main${page}`);

      await screen.findByRole("heading", { level: 1, name: "sessions-cache" });
      expect(screen.getByRole("link", { name: "Starter" })).toHaveAttribute(
        "href",
        "/keyvalue/red-main/info#compute",
      );
      const docs = screen.getByRole("link", { name: "View docs" });
      expect(docs).toHaveAttribute(
        "href",
        "https://bex.co/docs/platform/key-value",
      );
      expect(docs).toHaveAttribute("target", "_blank");
      expect(screen.getByRole("button", { name: "Connect" })).toBeEnabled();
    },
  );

  it("connects a private instance's services, and sends the external URL to Networking", async () => {
    const user = userEvent.setup();
    const router = await renderKeyValuePages(
      ROUTES,
      "/keyvalue/red-main/metrics",
    );
    await user.click(await screen.findByRole("button", { name: "Connect" }));

    expect(await screen.findByText("redis://red-main:6379")).toBeVisible();
    expect(screen.getByText("Internal Redis URL")).toBeVisible();
    await user.click(screen.getByRole("tab", { name: "External" }));
    const panel = screen.getByRole("tabpanel", { name: "External" });
    expect(panel).toHaveTextContent(
      "External Redis URLExternal traffic not allowed. Add IP addresses in the Networking section.",
    );
    expect(within(panel).queryByText("Redis CLI Command")).toBeNull();

    await user.click(within(panel).getByRole("link", { name: "Networking" }));

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/keyvalue/red-main/info"),
    );
    expect(router.state.location.hash).toBe("networking");
  });

  it("offers a public instance's external URL and bex kv-cli", async () => {
    kv.public = true;
    const user = userEvent.setup();
    await renderKeyValuePages(ROUTES, "/keyvalue/red-main");
    await user.click(await screen.findByRole("button", { name: "Connect" }));
    await user.click(screen.getByRole("tab", { name: "External" }));

    expect(
      await screen.findByText("rediss://red-main:pw@red-main.kv.bex.co:6379"),
    ).toBeVisible();
    expect(
      screen.getByRole("group", { name: "Redis CLI Command" }),
    ).toHaveTextContent("bex kv-cli red-main");
  });

  it("disables Connect for a member who cannot reveal secrets", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: "VIEWER",
        canViewSensitive: false,
        reasonKey: (action) =>
          action === "can_view_sensitive"
            ? "capabilities.reasonCanViewSensitive"
            : undefined,
      }),
    );
    await renderKeyValuePages(ROUTES, "/keyvalue/red-main");

    expect(
      await screen.findByRole("button", { name: "Connect" }),
    ).toBeDisabled();
  });

  it("reserves the ready header's boxes while it is pending", async () => {
    await renderKeyValuePages(ROUTES, "/keyvalue/red-main");
    const ready = headerGeometry(
      (await screen.findByRole("heading", { level: 1 })).closest(".border-b")!,
    );
    document.body.innerHTML = "";

    await renderDatastorePendingFrame("keyValue", "/keyvalue/red-main");
    const pending = headerGeometry(
      await waitFor(() => {
        const header = document.querySelector(
          '[data-skeleton-region="resource-header"]',
        );
        expect(header).not.toBeNull();
        return header!;
      }),
    );

    expect(pending).toEqual(ready);
  });
});
