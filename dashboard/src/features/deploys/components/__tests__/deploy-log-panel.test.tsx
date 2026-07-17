import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeployLogPanel } from "../deploy-log-panel";
import type { UseDeployLogsResult } from "../../hooks/use-deploy-logs";

const logState: UseDeployLogsResult = {
  lines: [],
  loading: false,
  error: undefined,
  buildStoreUnavailable: false,
  buildLiveStatus: "idle",
};

vi.mock("../../hooks/use-deploy-logs", () => ({
  useDeployLogs: () => logState,
}));

beforeEach(() => {
  logState.lines = [];
  logState.loading = false;
  logState.error = undefined;
  logState.buildStoreUnavailable = false;
  logState.buildLiveStatus = "idle";
});

describe("DeployLogPanel", () => {
  it("renders the explanatory log-store state on a build-log 503", () => {
    logState.buildStoreUnavailable = true;

    render(
      <DeployLogPanel
        resource="web"
        startTime="2026-07-14T00:00:00Z"
        endTime={undefined}
        hasPreDeploy={false}
        followBuild={false}
      />,
    );

    expect(
      screen.getByText("Historical build logs are unavailable"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "This dashboard isn't connected to durable log storage. Application logs can still appear for this deploy.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("No logs yet")).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText("Search logs…")).toBeInTheDocument();
  });

  it("keeps a successful empty range distinct from a store outage", () => {
    render(
      <DeployLogPanel
        resource="web"
        startTime="2026-07-14T00:00:00Z"
        endTime="2026-07-14T00:01:00Z"
        hasPreDeploy={false}
        followBuild={false}
      />,
    );

    expect(screen.getByText("No logs yet")).toBeInTheDocument();
    expect(
      screen.queryByText("Historical build logs are unavailable"),
    ).not.toBeInTheDocument();
  });

  it("renders a query failure instead of an empty state", () => {
    logState.error = new Error("logs query unavailable");

    render(
      <DeployLogPanel
        resource="web"
        startTime="2026-07-14T00:00:00Z"
        endTime={undefined}
        hasPreDeploy={false}
        followBuild={false}
      />,
    );

    expect(screen.getByText("logs query unavailable")).toBeInTheDocument();
    expect(screen.queryByText("No logs yet")).not.toBeInTheDocument();
  });

  it("reports a refused live build stream instead of leaving an empty pane unexplained", () => {
    logState.buildLiveStatus = "error";

    render(
      <DeployLogPanel
        resource="web"
        startTime="2026-07-14T00:00:00Z"
        endTime={undefined}
        hasPreDeploy={false}
        followBuild
      />,
    );

    expect(
      screen.getByText("Live tail disconnected — reconnecting…"),
    ).toBeInTheDocument();
  });
});
