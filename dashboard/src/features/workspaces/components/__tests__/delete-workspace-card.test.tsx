import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeleteWorkspaceCard } from "@/features/workspaces/components/delete-workspace-card";
import type { WorkspaceView } from "@/features/workspaces/types";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

const remove = vi.fn();
vi.mock("@/features/workspaces/hooks/use-delete-workspace", () => ({
  useDeleteWorkspace: () => ({ remove, busy: false, error: null }),
}));

const setCurrentWorkspaceId = vi.fn();
const refetch = vi.fn();
const workspaceState: { workspaces: WorkspaceView[] } = { workspaces: [] };
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ ...workspaceState, setCurrentWorkspaceId, refetch }),
}));

const WORKSPACE: WorkspaceView = {
  id: "tea-1",
  name: "acme-hq",
  plan: "hobby",
  role: "admin",
  createdAt: null,
};
const OTHER: WorkspaceView = {
  id: "tea-2",
  name: "acme-staging",
  plan: "pro",
  role: "admin",
  createdAt: null,
};

beforeEach(() => {
  mockNavigate.mockReset();
  remove.mockReset();
  remove.mockResolvedValue(true);
  setCurrentWorkspaceId.mockReset();
  refetch.mockReset();
  workspaceState.workspaces = [WORKSPACE, OTHER];
});

describe("DeleteWorkspaceCard — type-to-confirm guard (w6/m3/t004)", () => {
  it("keeps delete disabled until the typed text exactly matches the workspace name", async () => {
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    const input = screen.getByLabelText(/Type acme-hq to confirm/);
    const button = screen.getByRole("button", { name: "Delete Workspace" });
    expect(button).toBeDisabled();

    await user.type(input, "acme-h");
    expect(button).toBeDisabled();
    expect(remove).not.toHaveBeenCalled();

    await user.type(input, "q");
    expect(button).toBeEnabled();
  });

  it("a typo is a no-op, not a destroyed workspace", async () => {
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    await user.type(screen.getByLabelText(/Type acme-hq to confirm/), "acme-hq ");
    const button = screen.getByRole("button", { name: "Delete Workspace" });
    expect(button).toBeDisabled();

    await user.click(button);
    expect(remove).not.toHaveBeenCalled();
  });

  it("on success, switches to a remaining workspace and navigates home", async () => {
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    await user.type(screen.getByLabelText(/Type acme-hq to confirm/), "acme-hq");
    await user.click(screen.getByRole("button", { name: "Delete Workspace" }));

    expect(remove).toHaveBeenCalledWith("tea-1", "acme-hq", "acme-hq");
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-2");
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/" });
  });

  it("routes to /new/workspace when no workspace remains", async () => {
    workspaceState.workspaces = [WORKSPACE];
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    await user.type(screen.getByLabelText(/Type acme-hq to confirm/), "acme-hq");
    await user.click(screen.getByRole("button", { name: "Delete Workspace" }));

    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/new/workspace" });
  });
});
