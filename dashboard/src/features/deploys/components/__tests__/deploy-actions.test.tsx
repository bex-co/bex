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

const cancelDeploy = vi.fn();
const rollbackService = vi.fn();
const mutationOptions: Record<string, { refetchQueries?: string[] }> = {};
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
}));

function renderActions(status: string) {
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys/$deployId",
    component: () => (
      <DeployActions serviceId="web" deployId="dep-1" status={status} />
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

  it("rolls back a live deploy and navigates to the new deploy", async () => {
    rollbackService.mockResolvedValue({
      data: {
        rollbackService: { id: "dep-rollback", status: "update_in_progress" },
      },
    });
    const user = userEvent.setup();
    const router = renderActions("live");

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
    const router = renderActions("live");

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

  // w6/m45 t003: the header's status pill reads the `Server` query, which is
  // otherwise only polled every 30s — so a Cancel or Rollback that refetched
  // only Deploys/ServiceEvents left the header claiming "Building" next to a
  // "Canceled" latest-deploy chip on the very same page, on all three surfaces
  // that mount DeployActions, until a reload.
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
});
