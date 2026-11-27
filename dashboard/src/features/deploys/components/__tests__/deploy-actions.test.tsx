import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { bumpAccessGeneration } from "@/features/capabilities/lib/access-generation";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { DeployActions } from "../deploy-actions";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";

const cancelDeploy = vi.fn();
const rollbackService = vi.fn();
const mutationOptions: Record<string, { refetchQueries?: string[] }> = {};
const apolloQuery = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useMutation: vi.fn(
    (
      doc: { definitions?: Array<{ name?: { value?: string } }> },
      options: { refetchQueries?: string[] },
    ) => {
      const name = doc.definitions?.[0]?.name?.value ?? "";
      mutationOptions[name] = options;
      return name === "RollbackService"
        ? [rollbackService, { loading: false }]
        : [cancelDeploy, { loading: false }];
    },
  ),
  useApolloClient: () => ({ query: apolloQuery }),
  useQuery: vi.fn(),
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

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

const allowedSnapshot = toResourceSnapshot("tea-test", "web", [
  {
    action: "cancel_deploy",
    outcome: "allowed",
    reason: null,
    precondition: null,
  },
  { action: "rollback", outcome: "allowed", reason: null, precondition: null },
  { action: "deploy", outcome: "allowed", reason: null, precondition: null },
]);

const deniedCreateSnapshot = toResourceSnapshot("tea-test", "web", [
  {
    action: "cancel_deploy",
    outcome: "allowed",
    reason: null,
    precondition: null,
  },
  {
    action: "rollback",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
  { action: "deploy", outcome: "allowed", reason: null, precondition: null },
]);

let deployState = {
  status: "ready" as const,
  snapshot: allowedSnapshot,
  refresh: vi.fn().mockResolvedValue(undefined),
};

vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useDeployActions: () => deployState,
  useServerActions: () => deployState,
}));

function renderActions(
  status: string,
  commit?: {
    commitId?: string | null;
    trigger?: string | null;
    buildAvailable?: boolean | null;
    offerRollback?: boolean;
  },
) {
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys/$deployId",
    component: () => (
      <>
        <span data-testid="deploy-page" />
        <DeployActions
          serviceId="web"
          deployId="dep-1"
          status={status}
          commitId={commit?.commitId}
          trigger={commit?.trigger}
          buildAvailable={commit?.buildAvailable}
          offerRollback={commit?.offerRollback}
        />
      </>
    ),
  });
  // Rollback is a link to the deploy's Confirm Rollback page (w1/m646).
  const rollbackPage = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys/$deployId/rollback",
    component: () => <p>confirm rollback</p>,
  });
  const router = createRouter({
    routeTree: root.addChildren([route, rollbackPage]),
    history: createMemoryHistory({
      initialEntries: ["/services/web/deploys/dep-1"],
    }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

// A Deploys-list row: it shows DeployActions only while the deploy has an
// action, so the cancel's own result unmounts the control (w4/623).
function renderListRow(onCancel: (hide: () => void) => void) {
  let hide = () => {};
  function Row() {
    const [shown, setShown] = useState(true);
    hide = () => setShown(false);
    return shown ? (
      <DeployActions
        serviceId="web"
        deployId="dep-1"
        status="build_in_progress"
      />
    ) : (
      <p>canceled</p>
    );
  }
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys",
    component: Row,
  });
  const other = createRoute({
    getParentRoute: () => root,
    path: "/elsewhere",
    component: () => <p>elsewhere</p>,
  });
  const router = createRouter({
    routeTree: root.addChildren([route, other]),
    history: createMemoryHistory({ initialEntries: ["/services/web/deploys"] }),
    context: { client: {} as never, session: null },
  });
  cancelDeploy.mockImplementation(async () => {
    onCancel(hide);
    return { data: { cancelDeploy: { id: "dep-1", status: "canceled" } } };
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  const receivedAt = Date.now();
  for (const snapshot of [allowedSnapshot, deniedCreateSnapshot]) {
    snapshot.receivedAt = receivedAt;
  }
  toastError.mockReset();
  toastSuccess.mockReset();
  cancelDeploy.mockReset();
  rollbackService.mockReset();
  apolloQuery.mockReset();
  deployState = {
    status: "ready",
    snapshot: allowedSnapshot,
    refresh: vi.fn().mockResolvedValue(undefined),
  };
  apolloQuery.mockResolvedValue({
    data: {
      deployActions: [
        {
          action: "cancel_deploy",
          outcome: "allowed",
          reason: null,
          precondition: null,
        },
        {
          action: "rollback",
          outcome: "allowed",
          reason: null,
          precondition: null,
        },
      ],
    },
  });
});

describe("DeployActions", () => {
  it("cancels the current non-terminal deploy through the shared mutation", async () => {
    cancelDeploy.mockResolvedValue({
      data: { cancelDeploy: { id: "dep-1", status: "canceled" } },
    });
    const user = userEvent.setup();
    renderActions("update_in_progress");

    await user.click(
      await screen.findByRole("button", { name: /^Cancel deploy / }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(cancelDeploy).toHaveBeenCalledWith({
      variables: { serviceId: "web", deployId: "dep-1" },
    });
  });

  it("toasts a list cancel whose result unmounted the row's control (w4/623)", async () => {
    const user = userEvent.setup();
    renderListRow((hide) => act(() => hide()));

    await user.click(
      await screen.findByRole("button", { name: /^Cancel deploy / }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(await screen.findByText("canceled")).toBeInTheDocument();
    expect(toastSuccess).toHaveBeenCalledWith("Deploy canceled.");
  });

  it("does not toast a cancel resolved after a workspace switch", async () => {
    const user = userEvent.setup();
    renderListRow((hide) => {
      act(() => hide());
      bumpAccessGeneration();
    });

    await user.click(
      await screen.findByRole("button", { name: /^Cancel deploy / }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(await screen.findByText("canceled")).toBeInTheDocument();
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("does not toast a cancel resolved after the user left the service", async () => {
    const user = userEvent.setup();
    let router: ReturnType<typeof renderListRow> | null = null;
    router = renderListRow((hide) => {
      act(() => hide());
      // The test router is untyped against the app route tree.
      void router?.history.push("/elsewhere");
    });

    await user.click(
      await screen.findByRole("button", { name: /^Cancel deploy / }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(await screen.findByText("elsewhere")).toBeInTheDocument();
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  // w8/m156: the registry keeps a bounded history of builds, and a deploy
  // whose build is gone cannot be rolled back to.
  it("offers no Rollback on a deactivated deploy whose build is no longer stored", async () => {
    renderActions("deactivated", { buildAvailable: false });
    // The row is not cancelable either, so nothing renders for it.
    await vi.waitFor(() => {
      expect(
        screen.queryByRole("link", { name: /^Rollback to / }),
      ).not.toBeInTheDocument();
    });
    expect(
      screen.queryByRole("button", { name: /^Rollback to / }),
    ).not.toBeInTheDocument();
  });

  // w1/157: Render's cron job has no Rollback in its dashboard.
  it("offers no Rollback where the service's dashboard has none, and keeps Cancel", async () => {
    renderActions("deactivated", { offerRollback: false });
    // The page around the control has rendered, so its absence is real.
    await screen.findByTestId("deploy-page");
    expect(
      screen.queryByRole("link", { name: /^Rollback to / }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Rollback to / }),
    ).not.toBeInTheDocument();
    cleanup();
    renderActions("build_in_progress", { offerRollback: false });
    expect(
      await screen.findByRole("button", { name: /^Cancel / }),
    ).toBeInTheDocument();
  });

  it("keeps Rollback when the build is stored or its state is unknown", async () => {
    renderActions("deactivated", { buildAvailable: true });
    expect(
      await screen.findByRole("link", { name: /^Rollback to / }),
    ).toBeInTheDocument();
  });

  // Render's Rollback opens the deploy's Confirm Rollback page, which lists
  // what the rollback changes before anything runs (w1/m646).
  it("opens the deploy's Confirm Rollback page and dispatches nothing", async () => {
    const user = userEvent.setup();
    const router = renderActions("deactivated");

    await user.click(
      await screen.findByRole("link", { name: /^Rollback to / }),
    );

    await vi.waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/services/web/deploys/dep-1/rollback",
      );
    });
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(rollbackService).not.toHaveBeenCalled();
  });

  it("offers rollback for a deactivated deploy that previously went live", async () => {
    renderActions("deactivated");

    expect(
      await screen.findByRole("link", { name: /^Rollback to / }),
    ).toBeInTheDocument();
  });

  it("does not offer rollback on the current live deploy", async () => {
    renderActions("live");

    expect(
      screen.queryByRole("link", { name: /^Rollback to / }),
    ).not.toBeInTheDocument();
  });

  it("refetches the service header's own query after cancel", async () => {
    renderActions("update_in_progress");

    expect(mutationOptions.CancelDeploy?.refetchQueries).toEqual([
      "Server",
      "Deploys",
      "ServiceEvents",
    ]);
  });

  it("offers neither action for a failed deploy", async () => {
    renderActions("build_failed");

    expect(
      screen.queryByRole("link", { name: /^Rollback to / }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Cancel deploy / }),
    ).not.toBeInTheDocument();
  });

  it("offers no way to the rollback page when the role may not roll back", async () => {
    deployState = {
      status: "ready",
      snapshot: deniedCreateSnapshot,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    const user = userEvent.setup();
    const router = renderActions("deactivated");

    const btn = await screen.findByRole("button", { name: /^Rollback to / });
    expect(btn).toBeDisabled();
    expect(
      screen.queryByRole("link", { name: /^Rollback to / }),
    ).not.toBeInTheDocument();
    await user.click(btn);
    expect(router.state.location.pathname).toBe("/services/web/deploys/dep-1");
    expect(rollbackService).not.toHaveBeenCalled();
  });

  it("still enables cancel under billing when the server permits it", async () => {
    deployState = {
      status: "ready",
      snapshot: toResourceSnapshot("tea-test", "web", [
        {
          action: "cancel_deploy",
          outcome: "allowed",
          reason: null,
          precondition: null,
        },
        {
          action: "deploy",
          outcome: "allowed",
          reason: null,
          precondition: "billing_blocked",
        },
        {
          action: "rollback",
          outcome: "allowed",
          reason: null,
          precondition: "billing_blocked",
        },
      ]),
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    renderActions("update_in_progress");
    const btn = await screen.findByRole("button", { name: /^Cancel deploy / });
    expect(btn).not.toBeDisabled();
    expect(btn).not.toHaveAttribute("aria-disabled", "true");
  });

  // The cancel dialog says what happens to the live version and to saved
  // changes (w1/m152 t003, w4/334) but names no commit.
  it("leaves the cancel confirm copy free of the commit", async () => {
    const user = userEvent.setup();
    renderActions("update_in_progress", { commitId: "039c3471" });

    await user.click(
      await screen.findByRole("button", { name: /^Cancel deploy / }),
    );
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "The in-progress deploy will be stopped. If an earlier deploy succeeded, it stays live; otherwise nothing will be serving. Any changes you saved stay saved and ship with the next deploy.",
    );
    expect(dialog.textContent ?? "").not.toContain("039c347");
  });
});

// The cancel dialog is read at the moment someone is deciding whether cancel
// strands them, so its reassurance has to be true then. On a service's FIRST
// deploy nothing has ever served, and the unconditional "the last successful
// deploy remains live" was a promise the product could not keep — live on
// 2026-09-19 the service read Canceled afterwards with no revision ever served
// (w4/103).
describe("DeployActions — the cancel dialog tells the truth about what stays live", () => {
  it("warns that nothing is serving when canceling the first deploy", async () => {
    const user = userEvent.setup();
    renderActions("update_in_progress", { trigger: "create" });

    await user.click(
      await screen.findByRole("button", { name: /^Cancel deploy / }),
    );
    const dialog = await screen.findByRole("alertdialog");

    expect(
      within(dialog).getByText(/no version is live yet/i),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByText(/last successful deploy remains live/i),
    ).not.toBeInTheDocument();
  });

  // w4/334 (a893): a manual retry after canceled attempts is not a "create"
  // deploy, yet nothing has ever served. Without the service's history the
  // dialog cannot know, so every other trigger — and an unknown one — states
  // both outcomes instead of promising a live fallback.
  for (const trigger of [
    "api",
    "new_commit",
    "config_change",
    "deploy_hook",
    null,
    undefined,
  ]) {
    it(`states both outcomes for a ${String(trigger)} deploy`, async () => {
      const user = userEvent.setup();
      renderActions("update_in_progress", { trigger });

      await user.click(
        await screen.findByRole("button", { name: /^Cancel deploy / }),
      );
      const dialog = await screen.findByRole("alertdialog");

      expect(
        within(dialog).getByText(
          /if an earlier deploy succeeded, it stays live; otherwise nothing will be serving/i,
        ),
      ).toBeInTheDocument();
    });
  }
});

// w4/m141: a selected rollbackable row stays a target when the service-wide
// summary found none (w6/m143/t003); the page it opens rechecks the grant
// before dispatching (deploy-rollback.test.tsx).
describe("DeployActions — the Rollback link", () => {
  it("links an eligible row the service-wide summary missed", async () => {
    deployState.snapshot = toResourceSnapshot("tea-test", "web", [
      {
        action: "rollback",
        outcome: "allowed",
        reason: null,
        precondition: "no_eligible_rollback_target",
      },
    ]);
    renderActions("deactivated");

    expect(
      await screen.findByRole("link", { name: /^Rollback to / }),
    ).toHaveAttribute("href", "/services/web/deploys/dep-1/rollback");
  });

  it("names each row's controls after the deploy they act on", async () => {
    renderActions("deactivated", { commitId: "80b423bc1234" });
    expect(
      await screen.findByRole("link", {
        name: "Rollback to 80b423b (dep-1)",
      }),
    ).toBeInTheDocument();
  });

  // WCAG 2.5.3 Label in Name (w4/466): "click Rollback" must reach the button.
  it("starts the Rollback control's accessible name with its visible label", async () => {
    renderActions("deactivated", { commitId: "80b423bc1234" });
    const link = await screen.findByRole("link", { name: /^Rollback/ });
    expect(link).toHaveTextContent("Rollback");
    expect(link.getAttribute("aria-label")).toMatch(/^Rollback/);
  });
});
