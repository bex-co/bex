import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { DeployRollback } from "../deploy-rollback";
import { RollbackComparisonBody } from "../rollback-comparison";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";
import { ServiceBaseProvider } from "@/features/services/lib/service-base";
import { codedGraphQLError } from "@/test/mocks/apollo";
import type { RollbackComparisonQuery } from "@/graphql/definitions";

// The Confirm Rollback page (w1/m646, docs/render-artifacts/rollback-comparison.md).

const rollbackService = vi.fn();
const mutationOptions: Record<string, { refetchQueries?: string[] }> = {};
const apolloQuery = vi.fn();
let comparisonResult: {
  loading: boolean;
  error?: Error;
  data?: RollbackComparisonQuery;
} = { loading: true };
const comparisonOptions: Array<{ skip?: boolean }> = [];

vi.mock("@apollo/client/react", () => ({
  useMutation: vi.fn(
    (
      doc: { definitions?: Array<{ name?: { value?: string } }> },
      options: { refetchQueries?: string[] },
    ) => {
      mutationOptions[doc.definitions?.[0]?.name?.value ?? ""] = options;
      return [rollbackService, { loading: false }];
    },
  ),
  useQuery: vi.fn((_doc: unknown, options: { skip?: boolean }) => {
    comparisonOptions.push(options);
    return options.skip ? { loading: false } : comparisonResult;
  }),
  useApolloClient: () => ({ query: apolloQuery }),
}));

const toastError = vi.fn();
const toastSuccess = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    warning: vi.fn(),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

const ask = vi.fn();
vi.mock("@/common/providers/protected-retry-context", () => ({
  useAskForProtectedConfirmation: () => ask,
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

function rollbackRow(outcome: string, precondition: string | null = null) {
  return {
    action: "rollback",
    outcome,
    reason: outcome === "denied" ? "insufficient_permission" : null,
    precondition,
  };
}

let deployState = {
  status: "ready" as const,
  snapshot: toResourceSnapshot("tea-test", "web", [rollbackRow("allowed")]),
  refresh: vi.fn().mockResolvedValue(undefined),
};
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useDeployActions: () => deployState,
}));

type Snapshot = NonNullable<
  NonNullable<
    RollbackComparisonQuery["rollbackComparison"]
  >["deploy"]["serviceConfigSnapshot"]
>;

function snapshot(over: Partial<Snapshot> = {}): Snapshot {
  return {
    __typename: "ServiceConfigSnapshot",
    runCommand: "npm start",
    preDeployCommand: null,
    dockerCommand: null,
    healthCheckPath: "/healthz",
    instances: 1,
    plan: "starter",
    deletedEnvGroupCount: 0,
    envGroups: [],
    envVarChanges: {
      __typename: "EnvVarChanges",
      createdVarKeys: [],
      deletedVarKeys: [],
      updatedVarKeys: [],
      createdFileKeys: [],
      deletedFileKeys: [],
      updatedFileKeys: [],
    },
    ...over,
  };
}

function comparison(
  current: Snapshot | null,
  old: Snapshot | null,
  currentPlanLower = false,
): NonNullable<RollbackComparisonQuery["rollbackComparison"]> {
  return {
    __typename: "RollbackComparison",
    currentPlanLower,
    currentLiveDeploy: {
      __typename: "RollbackComparisonDeploy",
      id: "dep-live",
      serviceConfigSnapshot: current,
    },
    deploy: {
      __typename: "RollbackComparisonDeploy",
      id: "dep-1",
      serviceConfigSnapshot: old,
    },
  };
}

const group = (id: string, name: string) => ({
  __typename: "ServiceConfigSnapshotEnvGroup" as const,
  id,
  name,
});

describe("RollbackComparisonBody", () => {
  it("lists each changed setting in Render's order and words", () => {
    const current = snapshot({
      runCommand: "npm start -- --v2",
      healthCheckPath: "/healthz",
      instances: 3,
      envGroups: [group("evg-b", "shared-b")],
    });
    const old = snapshot({
      runCommand: "npm start -- --v1",
      healthCheckPath: null,
      instances: 1,
      envGroups: [group("evg-a", "shared-a")],
      deletedEnvGroupCount: 1,
      envVarChanges: {
        __typename: "EnvVarChanges",
        createdVarKeys: ["ZED", "ADDED"],
        deletedVarKeys: ["GONE"],
        updatedVarKeys: ["KEEP"],
        createdFileKeys: ["new.txt"],
        deletedFileKeys: ["gone.txt"],
        updatedFileKeys: ["a.txt"],
      },
    });
    render(
      <RollbackComparisonBody
        loading={false}
        error={false}
        data={comparison(current, old, true)}
      />,
    );

    const rows = screen.getAllByRole("heading", { level: 3 });
    expect(rows.map((row) => row.textContent)).toEqual([
      "Start Command",
      "Health Check Path",
      "Instances",
      "Environment Variables",
      "Environment Group",
      "Secret Files",
      "Plan",
    ]);
    const row = (label: string) =>
      screen.getByRole("heading", { name: label }).parentElement!;

    // A scalar row is the live value, removed, over the target's, added.
    const start = row("Start Command");
    expect(start.querySelector("del")).toHaveTextContent("npm start -- --v2");
    expect(start.querySelector("ins")).toHaveTextContent("npm start -- --v1");
    expect(row("Health Check Path").querySelector("ins")).toHaveTextContent(
      "none",
    );
    expect(row("Instances").querySelector("del")).toHaveTextContent("3");

    // One table row per key, sorted, every value masked.
    const table = within(row("Environment Variables")).getByRole("table");
    const keyRows = within(table).getAllByRole("row").slice(1);
    expect(keyRows.map((r) => r.querySelector("code")?.textContent)).toEqual([
      "ADDED",
      "GONE",
      "KEEP",
      "ZED",
    ]);
    const [added, gone, kept] = keyRows;
    expect(added.querySelector("ins")).not.toBeNull();
    expect(added.querySelector("del")).toBeNull();
    expect(gone.querySelector("del")).not.toBeNull();
    expect(gone.querySelector("ins")).toBeNull();
    expect(kept.querySelector("del")).not.toBeNull();
    expect(kept.querySelector("ins")).not.toBeNull();
    expect(table).toHaveTextContent("••••••••••••");

    expect(row("Environment Group")).toHaveTextContent(
      "The rollback will unlink environment group shared-b which was added to the service after this deploy." +
        "The rollback will link environment group shared-a which was removed from the service after this deploy." +
        "This deploy was linked to 1 environment group that has since been deleted. It cannot be restored." +
        "The rollback will use current keys and values for environment groups linked to this deploy.",
    );
    expect(within(row("Environment Group")).getByText("shared-b").tagName).toBe(
      "STRONG",
    );
    expect(row("Secret Files")).toHaveTextContent(
      "Secret file new.txt was added after this deploy. The rollback will ignore it." +
        "Secret file gone.txt was deleted after this deploy. The rollback will restore the file." +
        "Secret file a.txt was changed after this deploy. The rollback will revert to the original contents.",
    );
    expect(row("Plan")).toHaveTextContent(
      "Your current plan has lower CPU and memory allocations than the plan originally used for this deploy.",
    );
  });

  it("counts several deleted groups in the plural", () => {
    render(
      <RollbackComparisonBody
        loading={false}
        error={false}
        data={comparison(snapshot(), snapshot({ deletedEnvGroupCount: 2 }))}
      />,
    );
    expect(
      screen.getByText(/environment groups that have since been deleted/),
    ).toHaveTextContent(
      "This deploy was linked to 2 environment groups that have since been deleted. They cannot be restored.",
    );
  });

  it("says nothing changed when both deploys run the same configuration", () => {
    render(
      <RollbackComparisonBody
        loading={false}
        error={false}
        data={comparison(snapshot(), snapshot())}
      />,
    );
    expect(
      screen.getByText("No configuration changes since this deploy."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
  });

  // Render has one sentence for a failed read and for either side's missing
  // configuration, never an empty diff.
  it.each([
    ["the live deploy's configuration is missing", false, null, snapshot()],
    ["the target's configuration is missing", false, snapshot(), null],
    ["the read failed", true, snapshot(), snapshot()],
  ])("warns when %s", (_case, error, current, old) => {
    render(
      <RollbackComparisonBody
        loading={false}
        error={error}
        data={comparison(current, old)}
      />,
    );
    expect(
      screen.getByText(
        "Unable to load configuration for the current live deploy. Rolling back to this deploy could cause unanticipated configuration changes.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("No configuration changes since this deploy."),
    ).not.toBeInTheDocument();
  });

  it("shows a spinner in the same frame while it loads", () => {
    const { container } = render(
      <RollbackComparisonBody loading error={false} data={undefined} />,
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      "Loading the configuration changes",
    );
    expect(container.firstElementChild).toHaveClass("min-h-[9.25rem]");
  });
});

function renderPage(
  props: {
    status?: string;
    buildAvailable?: boolean | null;
    autoDeployOn?: boolean;
  } = {},
  base: "/services" | "/static" = "/services",
) {
  const root = createRootRoute();
  const page = createRoute({
    getParentRoute: () => root,
    path: `${base}/$serviceId/deploys/$deployId/rollback`,
    component: () => (
      <ServiceBaseProvider value={base}>
        <DeployRollback
          serviceId="web"
          deployId="dep-1"
          status={"status" in props ? props.status : "deactivated"}
          buildAvailable={props.buildAvailable}
          autoDeployOn={props.autoDeployOn}
        />
      </ServiceBaseProvider>
    ),
  });
  const deployPage = createRoute({
    getParentRoute: () => root,
    path: `${base}/$serviceId/deploys/$deployId`,
    component: () => <p>deploy page</p>,
  });
  const router = createRouter({
    routeTree: root.addChildren([page, deployPage]),
    history: createMemoryHistory({
      initialEntries: [`${base}/web/deploys/dep-1/rollback`],
    }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

const confirmButton = () =>
  screen.findByRole("button", { name: "Rollback to this deploy" });

beforeEach(() => {
  toastError.mockReset();
  toastSuccess.mockReset();
  rollbackService.mockReset();
  ask.mockReset();
  apolloQuery.mockReset();
  comparisonOptions.length = 0;
  comparisonResult = {
    loading: false,
    data: {
      rollbackComparison: comparison(
        snapshot({ runCommand: "npm start -- --v2" }),
        snapshot(),
      ),
    },
  };
  deployState = {
    status: "ready",
    snapshot: toResourceSnapshot("tea-test", "web", [rollbackRow("allowed")]),
    refresh: vi.fn().mockResolvedValue(undefined),
  };
  deployState.snapshot.receivedAt = Date.now();
  apolloQuery.mockResolvedValue({
    data: { deployActions: [rollbackRow("allowed")] },
  });
});

describe("DeployRollback", () => {
  it("shows Render's Confirm Rollback block with the comparison", async () => {
    renderPage();
    const section = await screen.findByRole("region", {
      name: "Confirm Rollback",
    });
    expect(section).toHaveTextContent(
      "Rolling back reverts certain configuration changes. Learn more.",
    );
    expect(
      within(section).getByRole("link", { name: "Learn more." }),
    ).toHaveAttribute("href", "https://bex.co/docs/platform/rollbacks");
    expect(
      within(section).getByRole("heading", { name: "Start Command" }),
    ).toBeInTheDocument();
    expect(comparisonOptions.at(-1)?.skip).toBe(false);
  });

  it("rolls back and opens the new deploy", async () => {
    rollbackService.mockResolvedValue({
      data: { rollbackService: { id: "dep-rollback" } },
    });
    const user = userEvent.setup();
    const router = renderPage();

    await user.click(await confirmButton());

    expect(rollbackService).toHaveBeenCalledWith({
      variables: { serviceId: "web", deployId: "dep-1" },
    });
    await vi.waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/services/web/deploys/dep-rollback",
      );
    });
    expect(mutationOptions.RollbackService?.refetchQueries).toEqual([
      "Server",
      "Deploys",
      "ServiceEvents",
    ]);
  });

  it("goes back to the deploy on Cancel", async () => {
    const user = userEvent.setup();
    const router = renderPage();
    await user.click(await screen.findByRole("button", { name: "Cancel" }));
    await vi.waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/services/web/deploys/dep-1",
      );
    });
    expect(rollbackService).not.toHaveBeenCalled();
  });

  // Render sends a deploy it cannot roll back to back to the deploy's page.
  it.each([
    ["the live deploy", { status: "live" }],
    ["a deploy whose build is gone", { buildAvailable: false }],
  ])("leaves the page for %s", async (_case, props) => {
    const router = renderPage(props);
    await vi.waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/services/web/deploys/dep-1",
      );
    });
    expect(comparisonOptions.every((o) => o.skip)).toBe(true);
  });

  it("waits for the deploy before reading the comparison", async () => {
    renderPage({ status: undefined });
    expect(
      await screen.findByText("Loading the configuration changes"),
    ).toBeInTheDocument();
    expect(await confirmButton()).toBeDisabled();
    expect(comparisonOptions.every((o) => o.skip)).toBe(true);
  });

  // w4/m176: a rollback swaps the running code, so a protected member refuses
  // with the phrase; the page asks for exactly that phrase and retries.
  it("retries a protected rollback with the server-issued phrase", async () => {
    rollbackService
      .mockRejectedValueOnce(
        codedGraphQLError("PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED", {
          confirm: "sudo repoint service web",
          name: "web",
        }),
      )
      .mockResolvedValueOnce({
        data: { rollbackService: { id: "dep-rollback" } },
      });
    ask.mockResolvedValue("sudo repoint service web");
    const user = userEvent.setup();
    const router = renderPage();

    await user.click(await confirmButton());

    await vi.waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/services/web/deploys/dep-rollback",
      );
    });
    expect(ask).toHaveBeenCalledWith("sudo repoint service web", "web");
    expect(rollbackService).toHaveBeenNthCalledWith(2, {
      variables: {
        serviceId: "web",
        deployId: "dep-1",
        confirm: "sudo repoint service web",
      },
    });
  });

  it("stays quiet when the protected phrase is dismissed", async () => {
    rollbackService.mockRejectedValueOnce(
      codedGraphQLError("PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED", {
        confirm: "sudo repoint service web",
      }),
    );
    ask.mockResolvedValue(null);
    const user = userEvent.setup();
    renderPage();

    await user.click(await confirmButton());

    await vi.waitFor(() => expect(ask).toHaveBeenCalled());
    expect(rollbackService).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  // Render shows a failed rollback under its buttons, and stays on the page.
  it("shows a failure under the buttons and stays", async () => {
    rollbackService.mockRejectedValue(new Error("the build is gone"));
    const user = userEvent.setup();
    const router = renderPage();

    await user.click(await confirmButton());

    expect(await screen.findByRole("alert")).toHaveTextContent("Error:");
    expect(router.state.location.pathname).toBe(
      "/services/web/deploys/dep-1/rollback",
    );
  });

  it("does not navigate when the rollback names no new deploy", async () => {
    rollbackService.mockResolvedValue({
      data: { rollbackService: { id: null } },
    });
    const user = userEvent.setup();
    const router = renderPage();

    await user.click(await confirmButton());

    await vi.waitFor(() => expect(rollbackService).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe(
      "/services/web/deploys/dep-1/rollback",
    );
  });

  it("dispatches nothing when the role may not roll back", async () => {
    deployState.snapshot = toResourceSnapshot("tea-test", "web", [
      rollbackRow("denied"),
    ]);
    deployState.snapshot.receivedAt = Date.now();
    const user = userEvent.setup();
    renderPage();

    const button = await confirmButton();
    expect(button).toBeDisabled();
    await user.click(button);
    expect(rollbackService).not.toHaveBeenCalled();
  });

  // w6/m143/t003, w4/m141: the selected deploy stays a target when the
  // service-wide summary found none, at the button and at the recheck.
  it("rolls back a selected deploy the service-wide summary missed", async () => {
    deployState.snapshot = toResourceSnapshot("tea-test", "web", [
      rollbackRow("allowed", "no_eligible_rollback_target"),
    ]);
    deployState.snapshot.receivedAt = Date.now();
    apolloQuery.mockResolvedValue({
      data: {
        deployActions: [rollbackRow("allowed", "no_eligible_rollback_target")],
      },
    });
    rollbackService.mockResolvedValue({
      data: { rollbackService: { id: "dep-9" } },
    });
    const user = userEvent.setup();
    renderPage();

    await user.click(await confirmButton());

    await vi.waitFor(() => expect(rollbackService).toHaveBeenCalledTimes(1));
    expect(toastError).not.toHaveBeenCalled();
  });

  it("says why when the recheck refuses", async () => {
    apolloQuery.mockResolvedValue({
      data: { deployActions: [rollbackRow("allowed", "suspended")] },
    });
    const user = userEvent.setup();
    renderPage();

    await user.click(await confirmButton());

    await vi.waitFor(() => expect(toastError).toHaveBeenCalledTimes(1));
    expect(String(toastError.mock.calls[0][0])).not.toBe("");
    expect(rollbackService).not.toHaveBeenCalled();
  });

  it("warns that auto-deploy will be turned off only while it is on", async () => {
    renderPage({ autoDeployOn: true });
    expect(
      await screen.findByText(
        "will be disabled for this service to prevent accidental deploys.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Auto-Deploy")).toBeInTheDocument();
  });

  it("leaves the auto-deploy warning out when it is off", async () => {
    renderPage({ autoDeployOn: false });
    await confirmButton();
    expect(screen.queryByText("Auto-Deploy")).not.toBeInTheDocument();
    expect(screen.queryByText(/redirects, rewrites/)).not.toBeInTheDocument();
  });

  it("tells a static site its redirects, rewrites and headers stay", async () => {
    renderPage({}, "/static");
    const what = await screen.findByText("redirects, rewrites, and headers");
    expect(what.parentElement).toHaveTextContent(
      "Changes made to redirects, rewrites, and headers will not be rolled back.",
    );
  });
});
