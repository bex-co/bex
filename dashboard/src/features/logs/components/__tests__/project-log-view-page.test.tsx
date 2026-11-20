import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { ProjectLogViewPage } from "../project-log-view-page";
import type { ProjectLogViewerProps } from "../project-log-viewer";
import type { SavedSearch } from "@/features/projects/hooks/use-saved-searches";
import { SavedSearchType } from "@/graphql/definitions";
import { parseRangePreset } from "@/features/metrics/lib/range";

// A project's observability view page (w3/m505): the view comes from the
// project's saved searches by the URL's id, its name heads the page, its
// filter document is the scope, and its timeRange the default range.

const savedSearches: {
  views: SavedSearch[];
  loading: boolean;
  error: Error | undefined;
} = { views: [], loading: false, error: undefined };
vi.mock("@/features/projects/hooks/use-saved-searches", () => ({
  useSavedSearches: () => savedSearches,
}));
// The header's view actions (w3/m507) have their own test; here they only
// show which view they act on, and can report one leaving.
const leaving: { report?: (id: string) => void } = {};
vi.mock("../observability-view-actions", () => ({
  ObservabilityViewActions: ({
    view,
    onLeaving,
  }: {
    view: SavedSearch;
    onLeaving: (id: string) => void;
  }) => {
    leaving.report = onLeaving;
    return <span>actions for {view.name}</span>;
  },
}));
const viewerProps: ProjectLogViewerProps[] = [];
vi.mock("../project-log-viewer", () => ({
  ProjectLogViewer: (props: ProjectLogViewerProps) => {
    viewerProps.push(props);
    return (
      <button
        type="button"
        onClick={() => props.onRangeChange(parseRangePreset("24h")!)}
      >
        viewer
      </button>
    );
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

const APP_FILTERS = {
  filters: [],
  version: 1,
  timeRange: { relative: "4h" },
};

beforeEach(() => {
  viewerProps.length = 0;
  savedSearches.loading = false;
  savedSearches.error = undefined;
  savedSearches.views = [
    view(
      "svs-app",
      "Application Logs",
      SavedSearchType.ApplicationLogs,
      APP_FILTERS,
    ),
    view("svs-req", "Request Logs", SavedSearchType.RequestLogs),
  ];
});

describe("ProjectLogViewPage", () => {
  it("heads the page with the view and reads its scope at its default range", () => {
    const onSearchChange = vi.fn();
    render(
      <ProjectLogViewPage
        projectId="prj-observed"
        viewType={SavedSearchType.ApplicationLogs}
        search={{ savedSearchId: "svs-app" }}
        onSearchChange={onSearchChange}
      />,
    );
    expect(screen.getByText("Observability view")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Application Logs" }),
    ).toBeInTheDocument();
    const props = viewerProps.at(-1)!;
    expect(props.scope).toEqual({
      project: "prj-observed",
      filterDocument: APP_FILTERS,
    });
    expect(props.logType).toBe("app");
    expect(props.range.id).toBe("4h");

    fireEvent.click(screen.getByRole("button", { name: "viewer" }));
    expect(onSearchChange).toHaveBeenCalledWith({
      savedSearchId: "svs-app",
      r: "24h",
      rangeStart: undefined,
      rangeEnd: undefined,
    });
  });

  it("lets the URL's range win over the view's", () => {
    render(
      <ProjectLogViewPage
        projectId="prj-observed"
        viewType={SavedSearchType.ApplicationLogs}
        search={{ savedSearchId: "svs-app", r: "7d" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(viewerProps.at(-1)!.range.id).toBe("7d");
  });

  it("reads Request Logs as request lines", () => {
    render(
      <ProjectLogViewPage
        projectId="prj-observed"
        viewType={SavedSearchType.RequestLogs}
        search={{ savedSearchId: "svs-req" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(viewerProps.at(-1)!.logType).toBe("request");
    // No timeRange: the Logs tab's one-hour default, Render's r=1h.
    expect(viewerProps.at(-1)!.range.id).toBe("1h");
  });

  it("says a view of another type or id is not found, and links the project's own", () => {
    render(
      <ProjectLogViewPage
        projectId="prj-observed"
        viewType={SavedSearchType.RequestLogs}
        search={{ savedSearchId: "svs-app" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(screen.getByText("View not found")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Open Request Logs" }),
    ).toHaveAttribute(
      "href",
      "/project/prj-observed/observability/logs/request?savedSearchId=svs-req",
    );
    expect(viewerProps).toHaveLength(0);
  });

  it("tells a failed read from a missing view", () => {
    savedSearches.views = [];
    savedSearches.error = new Error("boom");
    render(
      <ProjectLogViewPage
        projectId="prj-observed"
        viewType={SavedSearchType.ApplicationLogs}
        search={{ savedSearchId: "svs-app" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(screen.getByText("Couldn't load this view")).toBeInTheDocument();
    expect(screen.queryByText("View not found")).not.toBeInTheDocument();
  });

  it("holds the page's own skeleton while the views load", () => {
    savedSearches.loading = true;
    savedSearches.views = [];
    const { container } = render(
      <ProjectLogViewPage
        projectId="prj-observed"
        viewType={SavedSearchType.ApplicationLogs}
        search={{ savedSearchId: "svs-app" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(
      container.querySelector('[data-route-skeleton="project-log-view"]'),
    ).not.toBeNull();
    for (const region of ["page-header", "log-toolbar", "log-panel"]) {
      expect(
        container.querySelector(`[data-skeleton-region="${region}"]`),
      ).not.toBeNull();
    }
  });

  it("carries the view's actions, and holds its skeleton while the view is deleted", () => {
    const { container, rerender } = render(
      <ProjectLogViewPage
        projectId="prj-observed"
        viewType={SavedSearchType.ApplicationLogs}
        search={{ savedSearchId: "svs-app" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(
      screen.getByText("actions for Application Logs"),
    ).toBeInTheDocument();

    // The trash button deleted the view: it leaves the list before the page
    // navigates away, and the page must not flash "View not found".
    act(() => leaving.report!("svs-app"));
    savedSearches.views = savedSearches.views.filter((v) => v.id !== "svs-app");
    rerender(
      <ProjectLogViewPage
        projectId="prj-observed"
        viewType={SavedSearchType.ApplicationLogs}
        search={{ savedSearchId: "svs-app" }}
        onSearchChange={() => undefined}
      />,
    );
    expect(screen.queryByText("View not found")).not.toBeInTheDocument();
    expect(
      container.querySelector('[data-route-skeleton="project-log-view"]'),
    ).not.toBeNull();
  });
});
