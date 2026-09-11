import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceRowActions } from "@/features/services/components/service-row-actions";
import type { ServiceView } from "@/features/services/types";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";

vi.mock("@/features/projects/hooks/use-move-to-project", () => ({
  useMoveToProject: () => ({
    projects: [],
    currentProjectId: () => null,
    moveTo: vi.fn(),
    removeFromProject: vi.fn(),
    busyId: null,
  }),
}));

const apolloQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: apolloQuery }),
  useQuery: vi.fn(),
  useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

const allowedServer = toResourceSnapshot("tea-test", "app", [
  { action: "suspend", outcome: "allowed", reason: null, precondition: null },
  { action: "resume", outcome: "allowed", reason: null, precondition: null },
  { action: "restart", outcome: "allowed", reason: null, precondition: null },
]);
const allowedDeploy = toResourceSnapshot("tea-test", "app", [
  { action: "deploy", outcome: "allowed", reason: null, precondition: null },
  { action: "cancel_deploy", outcome: "allowed", reason: null, precondition: null },
  { action: "rollback", outcome: "allowed", reason: null, precondition: null },
]);
const deniedOperate = toResourceSnapshot("tea-test", "app", [
  {
    action: "suspend",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
  {
    action: "resume",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
  {
    action: "restart",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
]);
const deniedDeploy = toResourceSnapshot("tea-test", "app", [
  {
    action: "deploy",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
]);

let serverState = {
  status: "ready" as const,
  snapshot: allowedServer,
  refresh: vi.fn().mockResolvedValue(undefined),
};
let deployState = {
  status: "ready" as const,
  snapshot: allowedDeploy,
  refresh: vi.fn().mockResolvedValue(undefined),
};

vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useServerActions: () => serverState,
  useDeployActions: () => deployState,
}));

const service = {
  id: "app",
  name: "app",
  type: "web_service",
  suspended: false,
  phase: "Running",
  url: null,
  createdAt: null,
  replicas: 1,
  revision: "r1",
} as ServiceView;

beforeEach(() => {
  apolloQuery.mockReset();
  serverState = {
    status: "ready",
    snapshot: allowedServer,
    refresh: vi.fn().mockResolvedValue(undefined),
  };
  deployState = {
    status: "ready",
    snapshot: allowedDeploy,
    refresh: vi.fn().mockResolvedValue(undefined),
  };
  apolloQuery.mockResolvedValue({
    data: {
      serverActions: [
        { action: "suspend", outcome: "allowed", reason: null, precondition: null },
      ],
    },
  });
});

describe("ServiceRowActions", () => {
  it("retries suspend only after the exact protected-environment phrase", async () => {
    const onRun = vi
      .fn()
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo suspend service app",
      })
      .mockResolvedValueOnce({ status: "success" });
    const user = userEvent.setup();
    render(
      <ServiceRowActions service={service} pending={null} onRun={onRun} />,
    );

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    await user.click(screen.getByRole("menuitem", { name: "Suspend" }));
    const ordinaryConfirm = await screen.findByRole("alertdialog");
    await user.click(
      within(ordinaryConfirm).getByRole("button", { name: "Suspend" }),
    );

    expect(onRun).toHaveBeenNthCalledWith(1, "suspend", service);
    const protectedDialog = await screen.findByRole("dialog");
    expect(
      within(protectedDialog).getByText("sudo suspend service app"),
    ).toBeInTheDocument();
    const input = within(protectedDialog).getByLabelText("Sudo Command");
    const retry = within(protectedDialog).getByRole("button", {
      name: "Suspend",
    });
    await user.type(input, "sudo suspend service ap");
    expect(retry).toBeDisabled();
    await user.type(input, "p");
    expect(retry).toBeEnabled();
    await user.click(retry);

    expect(onRun).toHaveBeenNthCalledWith(
      2,
      "suspend",
      service,
      "sudo suspend service app",
    );
  });

  it("dispatches zero forbidden lifecycle mutations for a viewer", async () => {
    serverState = {
      status: "ready",
      snapshot: deniedOperate,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    deployState = {
      status: "ready",
      snapshot: deniedDeploy,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    const onRun = vi.fn();
    const user = userEvent.setup();
    render(
      <ServiceRowActions service={service} pending={null} onRun={onRun} />,
    );

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    const suspend = screen.getByRole("menuitem", { name: "Suspend" });
    expect(suspend).toHaveAttribute("aria-disabled", "true");
    await user.click(suspend);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(onRun).not.toHaveBeenCalled();
  });

  it("gates restart on the deploy verb, not server restart", async () => {
    serverState = {
      status: "ready",
      snapshot: allowedServer,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    deployState = {
      status: "ready",
      snapshot: deniedDeploy,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    const onRun = vi.fn();
    const user = userEvent.setup();
    render(
      <ServiceRowActions service={service} pending={null} onRun={onRun} />,
    );

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    const restart = screen.getByRole("menuitem", { name: "Restart" });
    expect(restart).toHaveAttribute("aria-disabled", "true");
    await user.click(restart);
    expect(onRun).not.toHaveBeenCalled();
  });
});
