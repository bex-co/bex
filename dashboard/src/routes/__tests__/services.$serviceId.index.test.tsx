import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { ServiceOverviewPage } from "../services.$serviceId.index";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";

// The overview page is a pure client of useServer; drive its states directly.
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    replicas: 3,
    revision: "abc123",
    ...overrides,
  };
}

beforeEach(() => {
  serverState.service = null;
  serverState.loading = false;
  serverState.error = undefined;
});

describe("ServiceOverviewPage", () => {
  it("renders the live server(id) overview fields", () => {
    serverState.service = svc();
    render(<ServiceOverviewPage serviceId="app" />);

    // status badge (phase Running, not suspended) + verbatim phase row
    expect(screen.getAllByText("Running").length).toBeGreaterThanOrEqual(1);
    // url, replicas, revision, and the decoded suspended value all render
    expect(screen.getByText("https://app.onbex.co")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument(); // replicas
    expect(screen.getByText("abc123")).toBeInTheDocument(); // revision
    expect(screen.getByText("No")).toBeInTheDocument(); // suspended → No
  });

  it("shows the decoded suspended state as Yes for a suspended service", () => {
    serverState.service = svc({
      suspended: true,
      phase: "Hibernated",
      url: null,
    });
    render(<ServiceOverviewPage serviceId="app" />);

    // the string "suspended" enum decoded to a boolean → "Yes"
    expect(screen.getByText("Yes")).toBeInTheDocument();
    // the operator phase is still shown verbatim alongside it
    expect(screen.getByText("Hibernated")).toBeInTheDocument();
    // "Suspended" shows both as the status badge and the field label
    expect(screen.getAllByText("Suspended")).toHaveLength(2);
  });

  it("shows a skeleton while loading with no data", () => {
    serverState.loading = true;
    const { container } = render(<ServiceOverviewPage serviceId="app" />);
    // no field values yet, and no error/not-found copy
    expect(screen.queryByText("Service not found")).not.toBeInTheDocument();
    expect(
      container.querySelectorAll('[data-slot="skeleton"]').length,
    ).toBeGreaterThan(0);
  });

  it("shows an error alert when the query fails with no data", () => {
    serverState.error = new Error("network down");
    render(<ServiceOverviewPage serviceId="app" />);
    expect(screen.getByText("Couldn't load services")).toBeInTheDocument();
  });

  it("shows a not-found state when server(id) resolves nothing", () => {
    // not loading, no error, no service → the App doesn't exist
    render(<ServiceOverviewPage serviceId="ghost" />);
    expect(screen.getByText("Service not found")).toBeInTheDocument();
    expect(screen.getByText(/ghost/)).toBeInTheDocument();
  });
});
