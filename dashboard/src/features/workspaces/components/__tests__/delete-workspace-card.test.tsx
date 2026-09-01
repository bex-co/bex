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

// Render's live guard: the full "sudo delete workspace <name>" phrase, not the
// bare name (w6/m5/t002, docs/render-artifacts/workspace-lifecycle.md).
const PHRASE = "sudo delete workspace acme-hq";

describe("DeleteWorkspaceCard — sudo-phrase confirm guard (w6/m3/t004, w6/m5/t002)", () => {
  it("keeps delete disabled until the full sudo phrase is typed (bare name is not enough)", async () => {
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    // The exact phrase is body copy; the input itself is labeled "Sudo Command".
    expect(screen.getByText(PHRASE)).toBeInTheDocument();
    const input = screen.getByLabelText("Sudo Command");
    const button = screen.getByRole("button", { name: "Delete Workspace" });
    expect(button).toBeDisabled();

    // The bare workspace name is deliberately insufficient.
    await user.type(input, "acme-hq");
    expect(button).toBeDisabled();
    expect(remove).not.toHaveBeenCalled();

    await user.clear(input);
    await user.type(input, PHRASE);
    expect(button).toBeEnabled();
  });

  it("a typo is a no-op, not a destroyed workspace", async () => {
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    await user.type(screen.getByLabelText("Sudo Command"), `${PHRASE} `);
    const button = screen.getByRole("button", { name: "Delete Workspace" });
    expect(button).toBeDisabled();

    await user.click(button);
    expect(remove).not.toHaveBeenCalled();
  });

  it("on success, switches to a remaining workspace and navigates home", async () => {
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    await user.type(screen.getByLabelText("Sudo Command"), PHRASE);
    await user.click(screen.getByRole("button", { name: "Delete Workspace" }));

    expect(remove).toHaveBeenCalledWith("tea-1", "acme-hq", PHRASE);
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-2");
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/", replace: true });
  });

  it("routes to /new/workspace when no workspace remains", async () => {
    workspaceState.workspaces = [WORKSPACE];
    const user = userEvent.setup();
    render(<DeleteWorkspaceCard workspace={WORKSPACE} />);

    await user.type(screen.getByLabelText("Sudo Command"), PHRASE);
    await user.click(screen.getByRole("button", { name: "Delete Workspace" }));

    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(mockNavigate).toHaveBeenCalledWith({
      to: "/new/workspace",
      replace: true,
      search: { attempt: undefined },
    });
  });
});
