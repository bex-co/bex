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
vi.mock("@apollo/client/react", () => ({
  useMutation: vi.fn(
    (doc: { definitions?: Array<{ name?: { value?: string } }> }) => {
      const name = doc.definitions?.[0]?.name?.value;
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

    await user.click(
      await screen.findByRole("button", { name: "Roll Back to This Deploy" }),
    );
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

  it("offers rollback for a deactivated deploy that previously went live", async () => {
    renderActions("deactivated");

    expect(
      await screen.findByRole("button", { name: "Roll Back to This Deploy" }),
    ).toBeInTheDocument();
  });

  it("offers neither action for a failed deploy", async () => {
    renderActions("build_failed");

    expect(
      screen.queryByRole("button", { name: "Roll Back to This Deploy" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Cancel" }),
    ).not.toBeInTheDocument();
  });
});
