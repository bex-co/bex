import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { ProjectMetricsViewPage } from "../project-metrics-view-page";
import type { ProjectMetricsViewProps } from "../project-metrics-view";
import type { SavedSearch } from "@/features/projects/hooks/use-saved-searches";
import { SavedSearchType } from "@/graphql/definitions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  ACTIVITY_REASON,
  billingCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// A project's Service Metrics view page (w3/m508): a METRICS saved search by
// its URL id, its filter document the charts' scope, a service Metrics page's
// 12-hour default, and a not-found state that links the project's own.

const savedSearches: { views: SavedSearch[]; loading: boolean } = {
  views: [],
  loading: false,
};
const useSavedSearches = vi.fn();
vi.mock("@/features/projects/hooks/use-saved-searches", () => ({
  useSavedSearches: (projectId: string) => {
    useSavedSearches(projectId);
    return { ...savedSearches, error: undefined };
  },
}));
vi.mock("@/features/logs/components/observability-view-actions", () => ({
  ObservabilityViewActions: ({ view }: { view: SavedSearch }) => (
    <span>actions for {view.name}</span>
  ),
}));
const viewProps: ProjectMetricsViewProps[] = [];
vi.mock("../project-metrics-view", () => ({
  ProjectMetricsView: (props: ProjectMetricsViewProps) => {
    viewProps.push(props);
    return <span>charts</span>;
  },
}));
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({
    to,
    params,
    search,
    children,
  }: {
    to: string;
    params: Record<string, string>;
    search: Record<string, string>;
    children: ReactNode;
  }) => (
    <a
      href={`${to.replace(/\$(\w+)/g, (_, key: string) => params[key])}?savedSearchId=${search.savedSearchId}`}
    >
      {children}
    </a>
  ),
}));

const view = (
  id: string,
  name: string,
  type: SavedSearchType,
  filters: Record<string, unknown> = { filters: [], version: 1 },
): SavedSearch => ({ __typename: "SavedSearch", id, name, type, filters });

const STAGING = {
  filters: [{ field: "ENVIRONMENT", operator: "IS", values: ["evm-staging"] }],
  version: 1,
};

beforeEach(() => {
  viewProps.length = 0;
  useSavedSearches.mockClear();
  savedSearches.loading = false;
  savedSearches.views = [
    view("svs-app", "Application Logs", SavedSearchType.ApplicationLogs),
    view("svs-met", "Staging metrics", SavedSearchType.Metrics, STAGING),
  ];
});

describe("ProjectMetricsViewPage", () => {
  it("charts the view's scope over a service Metrics page's default range", () => {
    render(
      <ProjectMetricsViewPage
        projectId="prj-observed"
        search={{ savedSearchId: "svs-met" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(
      screen.getByRole("heading", { name: "Staging metrics" }),
    ).toBeInTheDocument();
    expect(screen.getByText("actions for Staging metrics")).toBeInTheDocument();
    const props = viewProps.at(-1)!;
    expect(props.scope).toEqual({
      project: "prj-observed",
      filterDocument: STAGING,
    });
    expect(props.range.id).toBe("12h");
  });

  it("says a log view's id is not a metrics view, and links the project's own", () => {
    render(
      <ProjectMetricsViewPage
        projectId="prj-observed"
        search={{ savedSearchId: "svs-app" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(screen.getByText("View not found")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Open Staging metrics" }),
    ).toHaveAttribute(
      "href",
      "/project/prj-observed/observability/metrics?savedSearchId=svs-met",
    );
    expect(viewProps).toHaveLength(0);
  });

  it("holds its own skeleton while the views load", () => {
    savedSearches.loading = true;
    savedSearches.views = [];
    const { container } = render(
      <ProjectMetricsViewPage
        projectId="prj-observed"
        search={{ savedSearchId: "svs-met" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(
      container.querySelector('[data-route-skeleton="project-metrics-view"]'),
    ).not.toBeNull();
  });
});

// w9/m621: Render's role table gives the Billing role no "View service
// metrics"; bex-api refuses a project's metrics to it (can_view_activity). A
// Viewer reads the view as an Admin does.
describe("ProjectMetricsViewPage per role (w9/m621)", () => {
  it("a Billing member sees the page's title and why, and nothing is read", () => {
    vi.mocked(useCapabilities).mockReturnValue(billingCapabilities());
    render(
      <ProjectMetricsViewPage
        projectId="prj-observed"
        search={{ savedSearchId: "svs-met" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(
      screen.getByRole("heading", { name: "Service Metrics" }),
    ).toBeInTheDocument();
    expect(screen.getByText(ACTIVITY_REASON)).toBeInTheDocument();
    expect(screen.queryByText("charts")).not.toBeInTheDocument();
    expect(viewProps).toHaveLength(0);
    expect(useSavedSearches).not.toHaveBeenCalled();
  });

  it("a Viewer reads the view", () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    render(
      <ProjectMetricsViewPage
        projectId="prj-observed"
        search={{ savedSearchId: "svs-met" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(
      screen.getByRole("heading", { name: "Staging metrics" }),
    ).toBeInTheDocument();
    expect(screen.getByText("charts")).toBeInTheDocument();
    expect(screen.queryByText(ACTIVITY_REASON)).not.toBeInTheDocument();
  });
});
