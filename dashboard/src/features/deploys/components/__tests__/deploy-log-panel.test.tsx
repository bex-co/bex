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
      screen.getByText("Build logs need the log store."),
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Search logs…")).toBeInTheDocument();
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
