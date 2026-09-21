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

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

const allowedSnapshot = toResourceSnapshot("tea-test", "web", [
  { action: "cancel_deploy", outcome: "allowed", reason: null, precondition: null },
  { action: "rollback", outcome: "allowed", reason: null, precondition: null },
  { action: "deploy", outcome: "allowed", reason: null, precondition: null },
]);

const deniedCreateSnapshot = toResourceSnapshot("tea-test", "web", [
  { action: "cancel_deploy", outcome: "allowed", reason: null, precondition: null },
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
    commitMessage?: string | null;
    trigger?: string | null;
  },
) {
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys/$deployId",
    component: () => (
      <DeployActions
        serviceId="web"
        deployId="dep-1"
        status={status}
        commitId={commit?.commitId}
        commitMessage={commit?.commitMessage}
        trigger={commit?.trigger}
      />
    ),
  });
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({
      initialEntries: ["/services/web/deploys/dep-1"],
    }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
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
        { action: "cancel_deploy", outcome: "allowed", reason: null, precondition: null },
        { action: "rollback", outcome: "allowed", reason: null, precondition: null },
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

    await user.click(await screen.findByRole("button", { name: "Cancel" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(cancelDeploy).toHaveBeenCalledWith({
      variables: { serviceId: "web", deployId: "dep-1" },
    });
  });

  it("rolls back a deactivated deploy and navigates to the new deploy", async () => {
    rollbackService.mockResolvedValue({
      data: {
        rollbackService: { id: "dep-rollback", status: "update_in_progress" },
      },
    });
    const user = userEvent.setup();
    const router = renderActions("deactivated");

    await user.click(await screen.findByRole("button", { name: "Rollback" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(rollbackService).toHaveBeenCalledWith({
      variables: { serviceId: "web", deployId: "dep-1" },
    });
    await vi.waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/services/web/deploys/dep-rollback",
      );
    });
  });

  it("does not navigate when rollback omits the new deploy id", async () => {
    rollbackService.mockResolvedValue({
      data: { rollbackService: { id: null, status: "update_in_progress" } },
    });
    const user = userEvent.setup();
    const router = renderActions("deactivated");

    await user.click(await screen.findByRole("button", { name: "Rollback" }));
    await user.click(
      within(await screen.findByRole("alertdialog")).getByRole("button", {
        name: "Proceed",
      }),
    );

    await vi.waitFor(() => expect(rollbackService).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe("/services/web/deploys/dep-1");
  });

  it("offers rollback for a deactivated deploy that previously went live", async () => {
    renderActions("deactivated");

    expect(
      await screen.findByRole("button", { name: "Rollback" }),
    ).toBeInTheDocument();
  });

  it("does not offer rollback on the current live deploy", async () => {
    renderActions("live");

    expect(
      screen.queryByRole("button", { name: "Rollback" }),
    ).not.toBeInTheDocument();
  });

  it("refetches the service header's own query after cancel and rollback", async () => {
    renderActions("update_in_progress");

    for (const name of ["CancelDeploy", "RollbackService"]) {
      expect(mutationOptions[name]?.refetchQueries).toEqual([
        "Server",
        "Deploys",
        "ServiceEvents",
      ]);
    }
  });

  it("offers neither action for a failed deploy", async () => {
    renderActions("build_failed");

    expect(
      screen.queryByRole("button", { name: "Rollback" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Cancel" }),
    ).not.toBeInTheDocument();
  });

  it("dispatches zero rollback mutations when create is denied", async () => {
    deployState = {
      status: "ready",
      snapshot: deniedCreateSnapshot,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    const user = userEvent.setup();
    renderActions("deactivated");

    const btn = await screen.findByRole("button", { name: "Rollback" });
    expect(btn).toBeDisabled();
    await user.click(btn);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
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
    const btn = await screen.findByRole("button", { name: "Cancel" });
    expect(btn).not.toBeDisabled();
    expect(btn).not.toHaveAttribute("aria-disabled", "true");
  });

  // w4/m110 t003: the last checkpoint before an irreversible rollout must
  // answer "roll back to WHAT?". The row behind the dialog shows the commit;
  // for months the dialog did not.
  it("names the commit being restored in the rollback confirm dialog", async () => {
    const user = userEvent.setup();
    renderActions("deactivated", {
      commitId: "039c3471ad2f6c9f2d8f5ab0c8d4e1f2a3b4c5d6",
      commitMessage: "Add Render deployment configuration\n\nbody line",
    });

    await user.click(await screen.findByRole("button", { name: "Rollback" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("039c347 Add Render deployment configuration");
    // The subject only — never the commit body.
    expect(dialog).not.toHaveTextContent("body line");
    // No raw interpolation placeholder leaked through.
    expect(dialog.textContent ?? "").not.toContain("{commit}");
  });

  it("keeps the generic rollback body when the deploy has no commit", async () => {
    const user = userEvent.setup();
    renderActions("deactivated", { commitId: null, commitMessage: null });

    await user.click(await screen.findByRole("button", { name: "Rollback" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "The service will redeploy from the image used in this deploy.",
    );
    expect(dialog.textContent ?? "").not.toContain("built from");
  });

  // The cancel dialog is untouched by the commit threading.
  it("leaves the cancel confirm copy byte-identical", async () => {
    const user = userEvent.setup();
    renderActions("update_in_progress", {
      commitId: "039c3471",
      commitMessage: "Add Render deployment configuration",
    });

    await user.click(await screen.findByRole("button", { name: "Cancel" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "The in-progress deploy will be stopped. The last successful deploy remains live.",
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

    await user.click(await screen.findByRole("button", { name: "Cancel" }));
    const dialog = await screen.findByRole("alertdialog");

    expect(
      within(dialog).getByText(/no version is live yet/i),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByText(/last successful deploy remains live/i),
    ).not.toBeInTheDocument();
  });

  for (const trigger of ["api", "new_commit", "config_change", "deploy_hook"]) {
    it(`keeps the remains-live reassurance for a ${trigger} deploy`, async () => {
      const user = userEvent.setup();
      renderActions("update_in_progress", { trigger });

      await user.click(await screen.findByRole("button", { name: "Cancel" }));
      const dialog = await screen.findByRole("alertdialog");

      expect(
        within(dialog).getByText(/last successful deploy remains live/i),
      ).toBeInTheDocument();
    });
  }

  // A caller that does not know the trigger must get the pre-existing body,
  // not the scarier one — the fallback has to be the safe direction.
  it("falls back to the generic body when the trigger is unknown", async () => {
    const user = userEvent.setup();
    renderActions("update_in_progress");

    await user.click(await screen.findByRole("button", { name: "Cancel" }));
    const dialog = await screen.findByRole("alertdialog");

    expect(
      within(dialog).getByText(/last successful deploy remains live/i),
    ).toBeInTheDocument();
  });
});
