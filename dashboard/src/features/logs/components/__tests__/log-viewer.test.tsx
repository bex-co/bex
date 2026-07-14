import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { LogViewer } from "../log-viewer";
import type { UseLogHistoryResult } from "../../hooks/use-log-history";

// Drive the viewer's states by stubbing its data layer, mirroring the
// services.$serviceId routing test's approach.
const historyState: UseLogHistoryResult = {
  lines: [],
  loading: false,
  error: undefined,
  storeUnavailable: false,
};
vi.mock("../../hooks/use-log-history", () => ({
  useLogHistory: () => historyState,
}));
vi.mock("../../hooks/use-live-logs", () => ({
  useLiveLogs: () => ({ lines: [], status: "idle" }),
}));
vi.mock("../../hooks/use-log-label-values", () => ({
  useLogLabelValues: () => [],
}));

beforeEach(() => {
  historyState.lines = [];
  historyState.loading = false;
  historyState.error = undefined;
  historyState.storeUnavailable = false;
});

describe("LogViewer store-unavailable state (w5/008)", () => {
  it("shows the explanatory 'needs the log store' state on a 503", () => {
    historyState.storeUnavailable = true;
    render(<LogViewer resource="web" />);
    expect(
      screen.getByText("Request logs need the log store"),
    ).toBeInTheDocument();
    // Not the generic error title.
    expect(screen.queryByText("Couldn't load logs")).not.toBeInTheDocument();
  });

  it("shows the generic error state for a non-store error", () => {
    historyState.error = new Error("upstream exploded");
    render(<LogViewer resource="web" />);
    expect(screen.getByText("Couldn't load logs")).toBeInTheDocument();
    expect(screen.getByText("upstream exploded")).toBeInTheDocument();
  });

  it("renders log lines when history has content", () => {
    historyState.lines = [
      {
        key: "k1",
        timestamp: "t",
        time: "10:36:01",
        instance: "bv612",
        message: "hello from the app",
        type: "app",
        level: "",
        method: "",
        statusCode: "",
      },
    ];
    render(<LogViewer resource="web" />);
    expect(screen.getByText("hello from the app")).toBeInTheDocument();
  });
});
