import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { Route, ServiceMetricsPage } from "../services.$serviceId.metrics";
import {
  CLEARED_RANGE_SEARCH,
  type RangeSearch,
} from "@/features/metrics/lib/range";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";
import { useServiceEvents } from "@/features/events/hooks/use-service-events";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  ACTIVITY_REASON,
  billingCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// The page composes the two chart cards; their internals (Apollo queries,
// chart rendering) are covered by their own tests — stub them so this test
// stays about the w5/m48 type-gating decision: which cards a type gets.
vi.mock("@/features/metrics/components/application-metrics-card", () => ({
  ApplicationMetricsCard: ({
    runsInstances,
    shellEligible,
  }: {
    runsInstances: boolean;
    shellEligible?: boolean;
  }) => (
    <p>
      application card{runsInstances ? "" : " (parked)"}
      {shellEligible ? " (shell)" : ""}
    </p>
  ),
}));
vi.mock("@/features/metrics/components/network-metrics-card", () => ({
  NetworkMetricsCard: ({
    servesRequests,
    markers,
  }: {
    servesRequests?: boolean;
    markers?: unknown[];
  }) => (
    <p
      data-serves-requests={String(servesRequests)}
      data-markers={markers ? "yes" : "no"}
    >
      network card
    </p>
  ),
}));
vi.mock("@/features/metrics/components/metrics-filters", () => ({
  MetricsFilters: ({ events }: { events?: unknown }) => (
    <p data-events={events ? "yes" : "no"}>filters</p>
  ),
}));
vi.mock("@/features/events/components/event-timeline", () => ({
  EventTimeline: () => <p>timeline</p>,
}));
vi.mock("@/features/events/hooks/use-service-events", () => ({
  useServiceEvents: vi.fn(() => ({ events: [] })),
}));

const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

beforeEach(() => {
  serverState.service = null;
  serverState.loading = false;
  vi.mocked(useServiceEvents).mockClear();
});

// w9/m621: Render's role table gives the Billing role no "View service
// metrics"; bex-api refuses every metric read to it (can_view_activity). A
// Viewer reads the page as an Admin does.
describe("ServiceMetricsPage per role (w9/m621)", () => {
  const webService = {
    id: "srv-1",
    type: "web_service",
    suspended: false,
    phase: "Running",
  } as ServiceView;

  it("a Billing member sees why: no filter, card or chart marker reads", () => {
    vi.mocked(useCapabilities).mockReturnValue(billingCapabilities());
    serverState.service = webService;
    render(<ServiceMetricsPage serviceId="srv-1" />);

    expect(screen.getByText(ACTIVITY_REASON)).toBeInTheDocument();
    for (const part of ["filters", "application card", "network card"]) {
      expect(screen.queryByText(part)).not.toBeInTheDocument();
    }
    expect(useServiceEvents).not.toHaveBeenCalled();
  });

  it("a Viewer reads the charts and their event markers", () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    serverState.service = webService;
    render(<ServiceMetricsPage serviceId="srv-1" />);

    expect(screen.getByText("filters")).toBeInTheDocument();
    expect(screen.getByText("application card")).toBeInTheDocument();
    expect(screen.getByText("network card")).toBeInTheDocument();
    expect(screen.queryByText(ACTIVITY_REASON)).not.toBeInTheDocument();
    expect(useServiceEvents).toHaveBeenCalledWith("srv-1", expect.anything());
  });
});

describe("metrics range search contract (w6/065)", () => {
  const validate = Route.options.validateSearch as (
    search: Record<string, unknown>,
  ) => RangeSearch;

  it("round-trips a picked preset and a valid custom window through the URL", () => {
    expect(validate({ range: "30d" })).toEqual({ range: "30d" });
    expect(
      validate({
        range: "custom",
        rangeStart: "2026-07-01T00:00:00.000Z",
        rangeEnd: "2026-07-01T06:00:00.000Z",
      }),
    ).toEqual({
      range: "custom",
      rangeStart: "2026-07-01T00:00:00.000Z",
      rangeEnd: "2026-07-01T06:00:00.000Z",
    });
  });

  it("rejects malformed values so the 12h default still applies", () => {
    // Render's `t` comes back too, as a tombstone (w3/m707).
    const cleared = { ...CLEARED_RANGE_SEARCH, t: undefined };
    expect(validate({})).toStrictEqual(cleared);
    expect(validate({ range: "6h" })).toStrictEqual(cleared); // retired id
    expect(validate({ range: "custom" })).toStrictEqual(cleared); // no bounds
    // The Logs page's `r` Render alias is deliberately not honored here:
    // Render's Metrics names its window `t`.
    expect(validate({ r: "1h" })).toStrictEqual(cleared);
  });
});

describe("ServiceMetricsPage (w5/m48 — static sites get no pod metrics)", () => {
  it("shows both cards for a web service", () => {
    serverState.service = {
      id: "srv-1",
      type: "web_service",
      suspended: false,
      phase: "Running",
    } as ServiceView;
    render(<ServiceMetricsPage serviceId="srv-1" />);

    expect(screen.getByText("application card")).toBeInTheDocument();
    expect(screen.getByText("network card")).toBeInTheDocument();
  });

  // w4/717: Shell actions on Memory/CPU follow the Shell page and Logs tab's
  // eligibility rule (`Boolean(service.sshAddress)`).
  it("enables Shell actions on the Application card only when the service has an sshAddress (w4/717)", () => {
    serverState.service = {
      id: "srv-1",
      type: "web_service",
      suspended: false,
      phase: "Running",
      sshAddress: "srv-1@ssh.bex.co",
    } as ServiceView;
    render(<ServiceMetricsPage serviceId="srv-1" />);

    expect(screen.getByText("application card (shell)")).toBeInTheDocument();
  });

  // w5/m125: a parked service's current-limit read is empty for want of a pod,
  // so the card must not read it as "no limit configured".
  it("tells the Application card when the service is parked", () => {
    serverState.service = {
      id: "srv-1",
      type: "web_service",
      suspended: false,
      phase: "Hibernated",
    } as ServiceView;
    render(<ServiceMetricsPage serviceId="srv-1" />);

    expect(screen.getByText("application card (parked)")).toBeInTheDocument();
  });

  it("hides the Application (CPU/memory) card for a static site", () => {
    serverState.service = { id: "srv-1", type: "static_site" } as ServiceView;
    render(<ServiceMetricsPage serviceId="srv-1" />);

    expect(screen.queryByText("application card")).not.toBeInTheDocument();
    // Requests/bandwidth stay: the static site's Ingress routers attribute
    // Traefik series per App (docs/ADR029-static-sites.md).
    expect(screen.getByText("network card")).toBeInTheDocument();
  });

  it("holds the Application card until the type is known (never fires pod queries for a static site)", () => {
    serverState.service = null;
    serverState.loading = true;
    render(<ServiceMetricsPage serviceId="srv-1" />);

    expect(screen.queryByText("application card")).not.toBeInTheDocument();
    expect(screen.getByText("network card")).toBeInTheDocument();
  });

  // w4/m236 t007: when the service resolves after first render, the
  // Application card must appear exactly once — a duplicate sibling
  // key={serviceId} (Application + Network) left the overwritten fiber's DOM
  // mounted and frozen beside its fresh replacement.
  it("renders exactly one Application card when the service resolves after first render", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    try {
      serverState.service = null;
      serverState.loading = true;
      const { rerender } = render(<ServiceMetricsPage serviceId="srv-1" />);
      expect(screen.queryByText("application card")).not.toBeInTheDocument();

      serverState.service = {
        id: "srv-1",
        type: "web_service",
        suspended: false,
        phase: "Running",
      } as ServiceView;
      serverState.loading = false;
      rerender(<ServiceMetricsPage serviceId="srv-1" />);

      expect(screen.getAllByText("application card")).toHaveLength(1);
      expect(screen.getAllByText("network card")).toHaveLength(1);
      // The production duplicate was a fiber leaked by react-dom's keyed
      // reconciliation; React names the cause itself. The stubbed DOM does
      // not reproduce the leak, so pin the warning too: duplicate sibling
      // keys must never come back.
      const duplicateKeyWarning = consoleError.mock.calls.some((args) =>
        args.some((a) => typeof a === "string" && a.includes("same key")),
      );
      expect(duplicateKeyWarning).toBe(false);
    } finally {
      consoleError.mockRestore();
    }
  });
});

// w3/103: Render's cron job Metrics page (CronJobMetrics) is the range picker
// and Network Metrics → Outbound Bandwidth, with no event data for the chart.
describe("ServiceMetricsPage for a cron job (w3/103)", () => {
  it("shows the range picker and Outbound Bandwidth only", () => {
    serverState.service = {
      id: "crn-1",
      type: "cron_job",
      suspended: false,
      phase: "Running",
    } as ServiceView;
    render(<ServiceMetricsPage serviceId="crn-1" />);

    // The toolbar has no event filter or timeline toggle, so no timeline.
    expect(screen.getByText("filters")).toHaveAttribute("data-events", "no");
    expect(screen.queryByText("timeline")).not.toBeInTheDocument();
    expect(screen.queryByText(/application card/)).not.toBeInTheDocument();
    const network = screen.getByText("network card");
    expect(network).toHaveAttribute("data-serves-requests", "false");
    expect(network).toHaveAttribute("data-markers", "no");
    expect(useServiceEvents).not.toHaveBeenCalled();
  });

  it.each([
    ["web_service", "true"],
    ["private_service", "false"],
    ["background_worker", "false"],
  ])(
    "keeps a %s's event controls, markers and Application card",
    (type, servesRequests) => {
      serverState.service = {
        id: "srv-1",
        type,
        suspended: false,
        phase: "Running",
      } as ServiceView;
      render(<ServiceMetricsPage serviceId="srv-1" />);

      expect(screen.getByText("filters")).toHaveAttribute("data-events", "yes");
      expect(screen.getByText("application card")).toBeInTheDocument();
      const network = screen.getByText("network card");
      expect(network).toHaveAttribute("data-serves-requests", servesRequests);
      expect(network).toHaveAttribute("data-markers", "yes");
      expect(useServiceEvents).toHaveBeenCalledWith("srv-1", expect.anything());
    },
  );

  it("keeps a static site's event controls and markers", () => {
    serverState.service = { id: "srv-1", type: "static_site" } as ServiceView;
    render(<ServiceMetricsPage serviceId="srv-1" />);

    expect(screen.getByText("filters")).toHaveAttribute("data-events", "yes");
    expect(screen.getByText("network card")).toHaveAttribute(
      "data-markers",
      "yes",
    );
  });
});
