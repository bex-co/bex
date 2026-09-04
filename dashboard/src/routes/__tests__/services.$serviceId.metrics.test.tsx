import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { Route, ServiceMetricsPage } from "../services.$serviceId.metrics";
import type { RangeSearch } from "@/features/metrics/lib/range";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";

// The page composes the two chart cards; their internals (Apollo queries,
// chart rendering) are covered by their own tests — stub them so this test
// stays about the w5/m48 type-gating decision: which cards a type gets.
vi.mock("@/features/metrics/components/application-metrics-card", () => ({
  ApplicationMetricsCard: () => <p>application card</p>,
}));
vi.mock("@/features/metrics/components/network-metrics-card", () => ({
  NetworkMetricsCard: () => <p>network card</p>,
}));
vi.mock("@/features/metrics/components/metrics-filters", () => ({
  MetricsFilters: () => <p>filters</p>,
}));
vi.mock("@/features/events/components/event-timeline", () => ({
  EventTimeline: () => <p>timeline</p>,
}));
vi.mock("@/features/events/hooks/use-service-events", () => ({
  useServiceEvents: () => ({ events: [] }),
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

  it("drops malformed values so the 12h default still applies", () => {
    expect(validate({})).toEqual({});
    expect(validate({ range: "6h" })).toEqual({}); // retired id
    expect(validate({ range: "custom" })).toEqual({}); // no bounds
    // The Logs page's `r` Render alias is deliberately not honored here.
    expect(validate({ r: "1h" })).toEqual({});
  });
});

describe("ServiceMetricsPage (w5/m48 — static sites get no pod metrics)", () => {
  it("shows both cards for a web service", () => {
    serverState.service = { id: "srv-1", type: "web_service" } as ServiceView;
    render(<ServiceMetricsPage serviceId="srv-1" />);

    expect(screen.getByText("application card")).toBeInTheDocument();
    expect(screen.getByText("network card")).toBeInTheDocument();
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
});
