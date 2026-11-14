import { beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { formatDateTime } from "@/common/lib/format";
import { AgentRepoEnvironmentBuildsDialog } from "@/features/agent-environment/components/agent-repo-environment-builds-dialog";
import type {
  RepoEnvironmentBuildDetail,
  UseAgentRepoEnvironmentBuildResult,
  UseAgentRepoEnvironmentBuildsResult,
} from "@/features/agent-environment/hooks/use-agent-repo-environment-builds";
import type { RepoEnvironmentBuildView } from "@/features/agent-environment/lib/repo-environment-builds";
import type { RepoEnvironmentView } from "@/features/agent-environment/lib/repo-environments";

const hooks = vi.hoisted(() => ({
  list: null as unknown as UseAgentRepoEnvironmentBuildsResult,
  one: null as unknown as (
    id: string | null,
  ) => UseAgentRepoEnvironmentBuildResult,
  readIds: [] as (string | null)[],
}));
vi.mock(
  "@/features/agent-environment/hooks/use-agent-repo-environment-builds",
  () => ({
    useAgentRepoEnvironmentBuilds: () => hooks.list,
    useAgentRepoEnvironmentBuild: (_owner: string, id: string | null) => {
      hooks.readIds.push(id);
      return hooks.one(id);
    },
  }),
);

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const ENV: RepoEnvironmentView = {
  repo: "acme/api",
  version: "v2",
  setup: [
    { name: "deps", run: "npm ci" },
    { name: "build", run: "npm run build" },
  ],
  checks: [{ name: "test", run: "npm test" }],
  latestBuild: null,
};

function build(
  id: string,
  status: string,
  over: Partial<RepoEnvironmentBuildView> = {},
): RepoEnvironmentBuildView {
  return {
    __typename: "AgentRepoEnvironmentBuild",
    id,
    ownerId: "tea-1",
    repo: "acme/api",
    version: "v2",
    trigger: "manual",
    status,
    failureCode: null,
    failureReason: null,
    headSha: null,
    sizeBytes: null,
    imageAvailable: status === "success",
    createdAt: "2026-11-14T09:00:00Z",
    startedAt: null,
    finishedAt: null,
    ...over,
  };
}

const FAILED = build("arb-2", "failed", {
  trigger: "environment_saved",
  failureCode: "setup_failed",
  failureReason: 'setup step "build" exited 1',
  headSha: "0123456789abcdef",
  startedAt: "2026-11-14T10:00:00Z",
  finishedAt: "2026-11-14T10:03:12Z",
});
const READY = build("arb-1", "success", {
  trigger: "refresh",
  headSha: "fedcba9876543210",
  sizeBytes: 1.5 * 1024 ** 3,
  startedAt: "2026-11-13T10:00:00Z",
  finishedAt: "2026-11-13T10:04:00Z",
});

const STEPS: Record<string, RepoEnvironmentBuildDetail["steps"]> = {
  "arb-2": [
    {
      __typename: "AgentRepoEnvironmentBuildStep",
      name: "deps",
      exitCode: 0,
      durationMs: 12_400,
      output: "added 312 packages\n",
    },
    {
      __typename: "AgentRepoEnvironmentBuildStep",
      name: "build",
      exitCode: 1,
      durationMs: 64_000,
      output: "error TS2307: Cannot find module 'x'\nTOKEN=[REDACTED]",
    },
  ],
  "arb-1": [
    {
      __typename: "AgentRepoEnvironmentBuildStep",
      name: "deps",
      exitCode: 0,
      durationMs: 9_000,
      output: "",
    },
    {
      __typename: "AgentRepoEnvironmentBuildStep",
      name: "build",
      exitCode: null,
      durationMs: 0,
      output: "",
    },
  ],
};

function detail(view: RepoEnvironmentBuildView): RepoEnvironmentBuildDetail {
  return { ...view, steps: STEPS[view.id] ?? [] };
}

function setup({
  builds = [FAILED, READY],
  environment = ENV,
  canManage = true,
  list = {},
  one,
}: {
  builds?: RepoEnvironmentBuildView[];
  environment?: RepoEnvironmentView;
  canManage?: boolean;
  list?: Partial<UseAgentRepoEnvironmentBuildsResult>;
  one?: (id: string | null) => UseAgentRepoEnvironmentBuildResult;
} = {}) {
  const buildNow = vi.fn(async () => build("arb-3", "queued"));
  hooks.list = {
    builds,
    loading: false,
    error: undefined,
    retry: vi.fn(),
    buildNow,
    starting: false,
    ...list,
  };
  hooks.one =
    one ??
    ((id) => {
      const found = [...builds, build("arb-3", "queued")].find(
        (b) => b.id === id,
      );
      return {
        build: found ? detail(found) : null,
        loading: false,
        error: undefined,
        retry: vi.fn(),
      };
    });
  const onOpenChange = vi.fn();
  render(
    <AgentRepoEnvironmentBuildsDialog
      ownerId="tea-1"
      environment={environment}
      canManage={canManage}
      onOpenChange={onOpenChange}
    />,
  );
  return { buildNow, onOpenChange, user: userEvent.setup() };
}

const dialog = () => screen.getByTestId("agent-repo-environment-builds-dialog");
const buildNowButton = () =>
  within(dialog()).getByRole("button", { name: "Build now" });

describe("AgentRepoEnvironmentBuildsDialog (ADR111)", () => {
  beforeEach(() => {
    hooks.readIds = [];
    vi.mocked(toast.success).mockClear();
  });

  // ADR111 D6: the dashboard says beside Build now that files a step writes
  // stay in the snapshot, and how sessions use the builds (D4).
  it("warns beside Build now that the snapshot keeps the files the steps write", () => {
    setup();
    const bar = within(dialog()).getByTestId("build-now-bar");
    expect(
      within(bar).getByRole("button", { name: "Build now" }),
    ).toBeEnabled();
    expect(within(bar).getByTestId("build-now-note")).toHaveTextContent(
      "Files the setup steps write are kept in the snapshot, so don't write credentials to files. Secrets reach the steps as environment variables, which aren't kept.",
    );
    expect(
      within(dialog()).getByText(
        "Sessions start from the newest successful build whose setup steps match the current ones. When none matches, they run the setup steps at the start of each turn instead.",
      ),
    ).toBeInTheDocument();
  });

  it("queues a build, says so and opens its log", async () => {
    const { user, buildNow } = setup();
    await user.click(buildNowButton());
    expect(buildNow).toHaveBeenCalledTimes(1);
    expect(toast.success).toHaveBeenCalledWith("Queued a build of acme/api");
    expect(hooks.readIds.at(-1)).toBe("arb-3");
    expect(within(dialog()).getByTestId("build-log")).toHaveTextContent(
      "This build is waiting to start.",
    );
    expect(within(dialog()).queryByRole("alert")).toBeNull();
  });

  it("names when the next build may start once the repository hit its limit", async () => {
    const { user, buildNow } = setup();
    buildNow.mockRejectedValueOnce(
      codedGraphQLError("AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED", {
        repo: "acme/api",
        limit: 12,
        nextBuildAt: "2026-11-15T08:30:00Z",
      }),
    );
    await user.click(buildNowButton());
    const time = formatDateTime("2026-11-15T08:30:00Z", "en");
    expect(time).toBeTruthy();
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(
      `This repository already started 12 builds in the last 24 hours. The next build can start at ${time}.`,
    );
    expect(toast.success).not.toHaveBeenCalled();
  });

  it.each([
    [
      codedGraphQLError("AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED"),
      "already started 12 builds in the last 24 hours. Try again later.",
    ],
    [
      codedGraphQLError("AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD"),
      "This environment has no setup steps, so there is nothing to build.",
    ],
    [
      codedGraphQLError("AGENT_REPO_ENVIRONMENT_NOT_FOUND"),
      "This repository no longer has an environment",
    ],
    [
      codedGraphQLError("FORBIDDEN"),
      "Only workspace admins can start a build.",
    ],
    [
      codedGraphQLError("AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE"),
      "Environment builds aren't available on this deployment.",
    ],
    [
      codedGraphQLError("AGENT_REPO_ENVIRONMENTS_UNAVAILABLE"),
      "Environment builds aren't available on this deployment.",
    ],
    [new Error("Failed to fetch"), "Couldn't start the build. Try again."],
  ])("maps a refused Build now %#", async (error, copy) => {
    const { user, buildNow } = setup();
    buildNow.mockRejectedValueOnce(error);
    await user.click(buildNowButton());
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(copy);
    expect(toast.success).not.toHaveBeenCalled();
    // The log stays on the newest build.
    expect(hooks.readIds.at(-1)).toBe("arb-2");
  });

  it("keeps Build now disabled for a member, who still reads the builds", () => {
    setup({ canManage: false });
    expect(buildNowButton()).toBeDisabled();
    expect(
      within(dialog()).getByText("Only workspace admins can start a build."),
    ).toBeInTheDocument();
    expect(within(dialog()).getByTestId("build-now-note")).toBeInTheDocument();
    expect(within(dialog()).getAllByTestId("build-step")).toHaveLength(2);
  });

  it("disables Build now once the environment has no setup steps", () => {
    setup({ environment: { ...ENV, setup: [] } });
    expect(buildNowButton()).toBeDisabled();
    expect(
      within(dialog()).getByText(/there is nothing to build/),
    ).toBeInTheDocument();
  });

  it("shows the newest build's log: each step's exit, duration and redacted output", () => {
    setup();
    const header = within(dialog()).getByTestId("build-panel-header");
    expect(
      within(header).getByTestId("repo-environment-build-status"),
    ).toHaveTextContent("Failed");
    expect(header).toHaveTextContent("Setup steps changed");
    expect(within(dialog()).getByTestId("build-details")).toHaveTextContent(
      "took 3m 12s · 0123456",
    );
    // The failure in words, then the server's reason naming the step.
    const failure = within(dialog()).getByTestId("build-failure");
    expect(failure).toHaveTextContent("A setup step failed.");
    expect(failure).toHaveTextContent('setup step "build" exited 1');
    const steps = within(dialog()).getAllByTestId("build-step");
    expect(steps).toHaveLength(2);
    expect(steps[0]).toHaveTextContent("deps");
    expect(within(steps[0]!).getByTestId("build-step-exit")).toHaveTextContent(
      "exit 0",
    );
    expect(steps[0]).toHaveTextContent("12s");
    expect(steps[0]!.querySelector("pre")).toHaveTextContent(
      "added 312 packages",
    );
    expect(within(steps[1]!).getByTestId("build-step-exit")).toHaveTextContent(
      "exit 1",
    );
    expect(steps[1]).toHaveTextContent("1m 4s");
    expect(steps[1]!.querySelector("pre")?.textContent).toBe(
      "error TS2307: Cannot find module 'x'\nTOKEN=[REDACTED]",
    );
  });

  it("lists the history with status and times and opens an older build's log", async () => {
    const { user } = setup();
    const history = within(dialog()).getByRole("list", {
      name: "Build history",
    });
    const rows = within(history).getAllByRole("button");
    expect(rows.map((row) => row.getAttribute("aria-current"))).toEqual([
      "true",
      "false",
    ]);
    expect(rows[0]).toHaveTextContent("Failed");
    expect(rows[0]).toHaveTextContent("Setup steps changed");
    expect(rows[0]).toHaveTextContent("took 3m 12s");
    expect(rows[1]).toHaveTextContent("Ready");
    expect(rows[1]).toHaveTextContent("Daily refresh");
    expect(rows[1]).toHaveTextContent("took 4m");

    await user.click(rows[1]!);
    expect(hooks.readIds.at(-1)).toBe("arb-1");
    expect(rows[1]).toHaveAttribute("aria-current", "true");
    expect(within(dialog()).queryByTestId("build-failure")).toBeNull();
    expect(within(dialog()).getByTestId("build-details")).toHaveTextContent(
      "took 4m · fedcba9 · 1.5 GiB",
    );
    const steps = within(dialog()).getAllByTestId("build-step");
    expect(steps[0]).toHaveTextContent("No output.");
    // A step with no exit code was stopped or never started.
    expect(within(steps[1]!).getByTestId("build-step-exit")).toHaveTextContent(
      "stopped",
    );
  });

  it("says when a successful build's snapshot is gone", () => {
    setup({ builds: [{ ...READY, imageAvailable: false }] });
    expect(within(dialog()).getByTestId("build-details")).toHaveTextContent(
      "snapshot removed",
    );
  });

  it.each([
    ["queued", "This build is waiting to start."],
    ["building", "The setup steps are running."],
    ["cancelled", "a newer build replaced it, or its environment was deleted"],
  ])("says why a %s build has no log yet", (status, copy) => {
    setup({ builds: [build("arb-9", status)] });
    expect(within(dialog()).getByTestId("build-log")).toHaveTextContent(copy);
    expect(within(dialog()).queryByTestId("build-step")).toBeNull();
  });

  it("explains a repository that never built", () => {
    setup({ builds: [] });
    expect(within(dialog()).getByText("No builds yet.")).toBeInTheDocument();
    expect(within(dialog()).getByTestId("build-log")).toHaveTextContent(
      "This repository hasn't built yet.",
    );
  });

  it("offers a retry when the log cannot be read", async () => {
    const retry = vi.fn();
    const { user } = setup({
      one: () => ({
        build: null,
        loading: false,
        error: new Error("boom"),
        retry,
      }),
    });
    const log = within(dialog()).getByTestId("build-log");
    expect(within(log).getByRole("alert")).toHaveTextContent(
      "Couldn't load this build's log.",
    );
    await user.click(within(log).getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalled();
  });

  it("says when the deployment has no builds, with a retry", () => {
    setup({
      builds: [],
      list: {
        error: codedGraphQLError("AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE"),
      },
    });
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(
      "Environment builds aren't available on this deployment.",
    );
    expect(
      within(dialog()).getByRole("button", { name: "Retry" }),
    ).toBeInTheDocument();
  });

  // The skeleton shares the ready view's frame: the history and the log
  // panel at the same fixed heights, side by side from md (dashboard/AGENTS.md).
  it("previews the ready log view at its geometry while the history loads", () => {
    setup({ builds: [], list: { loading: true } });
    const skeleton = within(dialog()).getByTestId(
      "agent-repo-environment-builds-skeleton",
    );
    const regions = (root: HTMLElement) =>
      Array.from(
        root.querySelectorAll<HTMLElement>("[data-skeleton-region]"),
      ).map((el) => [el.dataset.skeletonRegion, el.className]);
    const pending = regions(skeleton);
    // Build now and its note do not wait for the history.
    expect(within(dialog()).getByTestId("build-now-note")).toBeInTheDocument();
    expect(
      skeleton.querySelector("[data-testid=build-panel-header]"),
    ).toHaveClass("h-14");
    cleanup();

    setup();
    const ready = within(dialog()).getByTestId("agent-repo-environment-builds");
    expect(regions(ready)).toEqual(pending);
    expect(pending.map(([name]) => name)).toEqual([
      "build-list",
      "build-panel",
    ]);
    expect(pending[0]![1]).toContain("h-40");
    expect(pending[0]![1]).toContain("md:h-96");
    expect(pending[1]![1]).toContain("h-80");
    expect(within(dialog()).getByTestId("build-panel-header")).toHaveClass(
      "h-14",
    );
    // A history row is the skeleton row's height.
    expect(within(ready).getAllByRole("button")[0]).toHaveClass("h-14");
  });

  it("closes from its footer", async () => {
    const { user, onOpenChange } = setup();
    const footerClose = within(dialog())
      .getAllByRole("button", { name: "Close" })
      .find((button) => button.textContent === "Close");
    await user.click(footerClose!);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
