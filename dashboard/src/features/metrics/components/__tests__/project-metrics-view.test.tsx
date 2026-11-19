import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { ProjectMetricsView } from "../project-metrics-view";
import {
  useProjectMetrics,
  type UseProjectMetricsResult,
} from "@/features/metrics/hooks/use-project-metrics";
import { parseRangePreset } from "@/features/metrics/lib/range";
import type { ProjectMetricResource } from "@/features/metrics/lib/project-metrics";
import type { MetricId } from "@/features/metrics/types";

// A project's Service Metrics view (w3/m508): one line per resource on every
// chart, a legend that hides a resource everywhere, Total Requests' group-by,
// and every state a read can end in, each told apart.

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }
});

vi.mock("@/features/metrics/hooks/use-project-metrics", () => ({
  useProjectMetrics: vi.fn(),
}));
vi.mock("@/features/metrics/hooks/use-metrics-retention", () => ({
  useMetricsRetentionBound: () => undefined,
}));
const workspace = vi.hoisted(() => ({ plan: "pro" }));
vi.mock("@/features/workspaces/context", () => ({
  useWorkspace: () => ({ currentWorkspace: { id: "tea-1", ...workspace } }),
}));

const mockUseProjectMetrics = vi.mocked(useProjectMetrics);

const SCOPE = {
  project: "prj-1",
  filterDocument: { filters: [], version: 1 },
};

const resource = (
  id: string,
  name: string,
  status = "CHARTED",
  type = "web_service",
): ProjectMetricResource => ({ id, name, type, environmentId: null, status });

const WEB = resource("srv-web", "web");
const API = resource("srv-api", "api");
const ADMIN = resource("srv-admin", "admin");

function result(
  over: Partial<UseProjectMetricsResult> = {},
): UseProjectMetricsResult {
  return {
    series: [],
    resources: [],
    loading: false,
    unavailable: false,
    storeUnavailable: false,
    throttled: false,
    error: undefined,
    degradedSources: [],
    ...over,
  };
}

const lineFor = (id: string, unit = "cpu") => ({
  unit,
  labels: { resource: id },
  points: [
    { timestamp: "2026-11-19T10:00:00Z", value: 0.1 },
    { timestamp: "2026-11-19T10:01:00Z", value: 0.2 },
  ],
});

/** Three web services on every chart, unless a test answers a chart itself. */
let answers: Partial<Record<MetricId, UseProjectMetricsResult>>;
beforeEach(() => {
  workspace.plan = "pro";
  mockUseProjectMetrics.mockReset();
  answers = {};
  mockUseProjectMetrics.mockImplementation(
    (_scope, metric) =>
      answers[metric] ??
      result({
        resources: [WEB, API, ADMIN],
        series: [lineFor("srv-web"), lineFor("srv-api"), lineFor("srv-admin")],
      }),
  );
});

function renderView() {
  return render(
    <ProjectMetricsView
      scope={SCOPE}
      range={parseRangePreset("1h")!}
      onRangeChange={() => {}}
    />,
  );
}

/** The lines a chart section draws, by stroke. */
function strokes(title: string): string[] {
  const section = screen.getByText(title).parentElement!.parentElement!;
  return [...section.querySelectorAll('svg path[stroke^="var(--color-"]')].map(
    (p) => p.getAttribute("stroke") ?? "",
  );
}

/** The newest options the view read a chart with. */
function latestOptions(metric: MetricId) {
  return mockUseProjectMetrics.mock.calls
    .filter(([, m]) => m === metric)
    .at(-1)?.[2];
}

describe("ProjectMetricsView", () => {
  it("draws three distinguishable lines on every chart and hides a resource everywhere from the legend", async () => {
    const user = userEvent.setup();
    renderView();
    const legend = screen.getByRole("group", {
      name: "Resources in this view",
    });
    expect(
      within(legend)
        .getAllByRole("button")
        .map((b) => b.textContent),
    ).toEqual(["web", "api", "admin"]);
    for (const title of [
      "CPU",
      "Memory",
      "Total Instances",
      "Total Requests",
    ]) {
      const lines = strokes(title);
      expect(new Set(lines).size, title).toBe(3);
    }
    await user.click(within(legend).getByRole("button", { name: "api" }));
    expect(within(legend).getByRole("button", { name: "api" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    for (const title of ["CPU", "Outbound Bandwidth"]) {
      expect(strokes(title)).toHaveLength(2);
      expect(strokes(title)).not.toContain("var(--color-violet-500)");
    }
    // The scope reaches every chart's read.
    expect(mockUseProjectMetrics.mock.calls[0][0]).toBe(SCOPE);
  });

  it("groups Total Requests by status code or not at all, Render's group-by", async () => {
    const user = userEvent.setup();
    renderView();
    expect(latestOptions("http_requests")?.groupBy).toBe("RESOURCE");
    await user.click(screen.getByRole("combobox", { name: "Group by" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Resource",
      "Status code",
      "All requests",
    ]);
    await user.click(screen.getByRole("option", { name: "Status code" }));
    expect(latestOptions("http_requests")?.groupBy).toBe("STATUS_CODE");
    // Only Total Requests regroups.
    expect(latestOptions("cpu")?.groupBy).toBeUndefined();
  });

  it("names a failed read, a type a chart has no data for and a resource with no data, never drawing them", () => {
    answers.cpu = result({
      resources: [WEB, resource("srv-api", "api", "FAILED"), ADMIN],
      series: [lineFor("srv-web")],
    });
    answers.http_requests = result({
      resources: [
        WEB,
        API,
        resource("srv-jobs", "jobs", "NOT_CHARTED", "background_worker"),
      ],
      series: [lineFor("srv-web", "count"), lineFor("srv-api", "count")],
    });
    renderView();
    expect(strokes("CPU")).toHaveLength(1);
    expect(
      screen.getByText("Couldn't load api; this chart leaves them out."),
    ).toHaveAttribute("role", "status");
    expect(
      screen.getByText("No data in this range for admin."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Not charted for jobs: this chart has no data for their type.",
      ),
    ).toBeInTheDocument();
  });

  it("says a view is past the resources one view charts, with both counts, instead of any chart", () => {
    const tooLarge = new CombinedGraphQLErrors({
      errors: [
        {
          message: "bad request: this view charts 23 resources",
          extensions: {
            code: "PROJECT_VIEW_TOO_LARGE",
            resources: 23,
            limit: 20,
          },
        },
      ],
    } as never);
    mockUseProjectMetrics.mockImplementation(() => result({ error: tooLarge }));
    renderView();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This view holds too many resources to chart",
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "It holds 23 resources, and a view charts at most 20.",
    );
    expect(screen.queryByText("Application Metrics")).not.toBeInTheDocument();
  });

  it("says a view with nothing in it has nothing to chart, even on Hobby", () => {
    workspace.plan = "hobby";
    mockUseProjectMetrics.mockImplementation((_scope, metric) =>
      metric === "http_latency"
        ? result({
            error: new CombinedGraphQLErrors({
              errors: [
                {
                  message:
                    "bad request: response latency metrics require the Pro plan",
                  extensions: {
                    code: "PLAN_FEATURE_REQUIRED",
                    feature: "latencyMetrics",
                  },
                },
              ],
            } as never),
          })
        : result(),
    );
    renderView();
    expect(
      screen.getByText("Nothing to chart in this view yet"),
    ).toBeInTheDocument();
    expect(screen.queryByText("CPU")).not.toBeInTheDocument();
  });

  it("shows Hobby the response-time plan notice, read once over a fixed window", () => {
    workspace.plan = "hobby";
    answers.http_latency = result({
      error: new CombinedGraphQLErrors({
        errors: [
          {
            message:
              "bad request: response latency metrics require the Pro plan",
            extensions: {
              code: "PLAN_FEATURE_REQUIRED",
              feature: "latencyMetrics",
            },
          },
        ],
      } as never),
    });
    renderView();
    expect(
      screen.getByText(
        "Response latency metrics require the Pro plan or higher. This workspace is on Hobby.",
      ),
    ).toBeInTheDocument();
    // No moving window: the refusal is not read again at every tick.
    expect(latestOptions("http_latency")?.startTime).toBeUndefined();
    expect(latestOptions("cpu")?.startTime).toBeDefined();
  });

  it("keeps the charts' frames while the first reads are in flight", () => {
    mockUseProjectMetrics.mockImplementation(() => result({ loading: true }));
    renderView();
    expect(screen.getByText("CPU")).toBeInTheDocument();
    expect(screen.getAllByRole("status", { name: /loading/i }).length).toBe(6);
  });
});
