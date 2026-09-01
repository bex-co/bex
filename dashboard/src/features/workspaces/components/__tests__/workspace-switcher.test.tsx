import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { WorkspaceSwitcher } from "@/features/workspaces/components/workspace-switcher";

function renderSwitcher() {
  return render(
    <SidebarProvider>
      <WorkspaceSwitcher />
    </SidebarProvider>,
  );
}

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

const setCurrentWorkspaceId = vi.fn();
const workspaceState: {
  workspaces: {
    id: string;
    name: string;
    plan: string;
    role: string;
    createdAt: string | null;
  }[];
  currentWorkspace: { id: string; name: string } | null;
  loading: boolean;
} = {
  workspaces: [],
  currentWorkspace: null,
  loading: false,
};
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ ...workspaceState, setCurrentWorkspaceId }),
}));

const WORKSPACES = [
  {
    id: "tea-1",
    name: "acme-hq",
    plan: "hobby",
    role: "admin",
    createdAt: null,
  },
  {
    id: "tea-2",
    name: "acme-staging",
    plan: "pro",
    role: "admin",
    createdAt: null,
  },
];

beforeEach(() => {
  mockNavigate.mockReset();
  setCurrentWorkspaceId.mockReset();
  workspaceState.workspaces = WORKSPACES;
  workspaceState.currentWorkspace = WORKSPACES[0];
  workspaceState.loading = false;
});

describe("WorkspaceSwitcher", () => {
  it("shows the current workspace's name as the trigger", () => {
    renderSwitcher();
    expect(screen.getByRole("button", { name: /acme-hq/ })).toBeInTheDocument();
  });

  it("lists every workspace and switches on click", async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    await user.click(
      await screen.findByRole("menuitem", { name: /acme-staging/ }),
    );

    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-2");
  });

  it("navigates to billing, settings, and + New Workspace", async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    await user.click(await screen.findByRole("menuitem", { name: "Billing" }));
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/billing" });

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Workspace Settings" }),
    );
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/workspace/settings" });

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    await user.click(
      await screen.findByRole("menuitem", { name: "+ New Workspace" }),
    );
    expect(mockNavigate).toHaveBeenCalledWith({
      to: "/new/workspace",
      search: { attempt: undefined },
    });
  });

  it("shows each workspace's plan as a sublabel", async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    expect(await screen.findByText("Hobby")).toBeInTheDocument();
    expect(screen.getByText("Pro")).toBeInTheDocument();
  });
});
