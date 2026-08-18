import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeployLogPanel } from "../deploy-log-panel";
import type { UseDeployLogsResult } from "../../hooks/use-deploy-logs";
import type { LogLine } from "@/features/logs/types";
import { needsAnsiParse, parseAnsi } from "@/features/logs/lib/ansi";
import { setupVirtualGeometry } from "@/test/virtual-geometry";

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

// The panel embeds the virtualized LogLineList (w9/m83): give jsdom the layout
// geometry the virtualizer needs, or the rendered rows never mount.
setupVirtualGeometry();

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

  it("filters to build or application lines via Render's type selector (w1/029)", async () => {
    const line = (key: string, message: string, type: string): LogLine => ({
      key,
      timestamp: "2026-07-17T20:16:14Z",
      time: "20:16:14",
      instance: "",
      message,
      spans: null,
      type,
      level: "",
      method: "",
      statusCode: "",
    });
    logState.lines = [
      line("1", "==> Build queued", "build"),
      line("2", "running migration", "predeploy"),
      line("3", "server listening", "app"),
    ];

    render(
      <DeployLogPanel
        resource="web"
        startTime="2026-07-14T00:00:00Z"
        endTime={undefined}
        hasPreDeploy
        followBuild={false}
      />,
    );
    const user = userEvent.setup();

    // Default: All logs — everything interleaved.
    expect(screen.getByText("==> Build queued")).toBeInTheDocument();
    expect(screen.getByText("server listening")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Log type" }));
    await user.click(screen.getByRole("menuitemradio", { name: "Build logs" }));
    expect(screen.getByText("==> Build queued")).toBeInTheDocument();
    expect(screen.queryByText("server listening")).not.toBeInTheDocument();
    expect(screen.queryByText("running migration")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Log type" }));
    await user.click(
      screen.getByRole("menuitemradio", { name: "Application logs" }),
    );
    // Application = app + pre-deploy, matching Render's two-bucket split.
    expect(screen.queryByText("==> Build queued")).not.toBeInTheDocument();
    expect(screen.getByText("server listening")).toBeInTheDocument();
    expect(screen.getByText("running migration")).toBeInTheDocument();
  });

  it("searches the displayed text, not the raw ANSI bytes", async () => {
    const esc = "\u001b";
    const line = (key: string, message: string): LogLine => ({
      key,
      timestamp: "2026-07-17T20:16:14Z",
      time: "20:16:14",
      instance: "",
      message,
      spans: needsAnsiParse(message) ? parseAnsi(message) : null,
      type: "build",
      level: "",
      method: "",
      statusCode: "",
    });
    // A colorized build line: the needle straddles the escape, so matching the
    // wire bytes would drop this row from its own search.
    logState.lines = [
      line(
        "1",
        `${esc}[2mmin-height: var(--feed-${esc}[33mreserve${esc}[39m);`,
      ),
      line("2", "unrelated line"),
    ];

    render(
      <DeployLogPanel
        resource="web"
        startTime="2026-07-14T00:00:00Z"
        endTime={undefined}
        hasPreDeploy={false}
        followBuild={false}
      />,
    );
    const user = userEvent.setup();

    await user.type(
      screen.getByPlaceholderText("Search logs…"),
      "feed-reserve",
    );

    await waitFor(() => {
      expect(screen.queryByText("unrelated line")).not.toBeInTheDocument();
    });
    expect(screen.getByText("reserve")).toBeInTheDocument();
    expect(screen.queryByText("No logs yet")).not.toBeInTheDocument();
  });

  it("shows a short clickable instance and exposes its removable exact filter", async () => {
    const selectedInstance = "eden-cms-v2-6f7d8f9c4b-bv612";
    logState.lines = [
      {
        key: "selected",
        timestamp: "2026-07-17T20:16:14Z",
        time: "20:16:14",
        instance: selectedInstance,
        message: "selected instance",
        spans: null,
        type: "app",
        level: "",
        method: "",
        statusCode: "",
      },
      {
        key: "other",
        timestamp: "2026-07-17T20:16:15Z",
        time: "20:16:15",
        instance: "eden-cms-v2-6f7d8f9c4b-r9x2p",
        message: "other instance",
        spans: null,
        type: "app",
        level: "",
        method: "",
        statusCode: "",
      },
    ];

    render(
      <DeployLogPanel
        resource="eden-cms-v2"
        startTime="2026-07-14T00:00:00Z"
        endTime={undefined}
        hasPreDeploy={false}
        followBuild={false}
      />,
    );
    const user = userEvent.setup();

    const instance = screen.getByRole("button", {
      name: `Filter logs by instance ${selectedInstance}`,
    });
    expect(instance).toHaveTextContent("[bv612]");
    expect(instance).toHaveAttribute("title", selectedInstance);

    await user.click(instance);
    expect(screen.getByText("selected instance")).toBeInTheDocument();
    expect(screen.queryByText("other instance")).not.toBeInTheDocument();
    expect(
      screen.getByText(`Instance: ${selectedInstance}`),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Remove Instance filter" }),
    );
    expect(screen.getByText("other instance")).toBeInTheDocument();
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
