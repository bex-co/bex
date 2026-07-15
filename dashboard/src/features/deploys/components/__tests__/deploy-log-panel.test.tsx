import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeployLogPanel } from "../deploy-log-panel";
import type { UseDeployLogsResult } from "../../hooks/use-deploy-logs";

const logState: UseDeployLogsResult = {
  lines: [],
  loading: false,
  error: undefined,
  buildStoreUnavailable: false,
};

vi.mock("../../hooks/use-deploy-logs", () => ({
  useDeployLogs: () => logState,
}));

beforeEach(() => {
  logState.lines = [];
  logState.loading = false;
  logState.error = undefined;
  logState.buildStoreUnavailable = false;
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
      />,
    );

    expect(
      screen.getByText("Build logs need the log store."),
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Search logs…")).toBeInTheDocument();
  });
});
