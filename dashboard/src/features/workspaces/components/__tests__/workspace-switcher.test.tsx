import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { WorkspaceSwitcher } from "@/features/workspaces/components/workspace-switcher";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import { organization } from "@/features/organizations/__tests__/org-fixtures";
import type { WorkspaceView } from "@/features/workspaces/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

function renderSwitcher() {
  return render(
    <SidebarProvider>
      <WorkspaceSwitcher />
    </SidebarProvider>,
  );
}

const mockNavigate = vi.fn();
let routeParams: Record<string, string> = {};
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
  useParams: () => routeParams,
}));

type SwitcherWorkspace = Pick<
  WorkspaceView,
  "id" | "name" | "plan" | "role" | "createdAt"
> &
  Partial<Pick<WorkspaceView, "organization" | "primary" | "privacy">>;

const setCurrentWorkspaceId = vi.fn();
const workspaceState: {
  workspaces: SwitcherWorkspace[];
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
  vi.mocked(useOrganizations).mockReturnValue({
    organizations: [],
  });
  mockNavigate.mockReset();
  routeParams = {};
  setCurrentWorkspaceId.mockReset();
  workspaceState.workspaces = WORKSPACES;
  workspaceState.currentWorkspace = WORKSPACES[0];
  workspaceState.loading = false;
});

describe("WorkspaceSwitcher", () => {
  // w4/440: the pending placeholder must occupy the loaded trigger's box, or
  // the sidebar header grows from 48 to 60px and every nav link jumps 12px.
  it("renders its pending placeholder in the loaded trigger's box", () => {
    workspaceState.workspaces = [];
    workspaceState.currentWorkspace = null;
    workspaceState.loading = true;
    const { container, rerender } = renderSwitcher();
    const pending = container.querySelector(
      '[data-slot="workspace-switcher-pending"]',
    );
    expect(pending).toHaveClass("h-8", "p-2", "gap-2");
    expect(pending?.className).not.toMatch(/\bp[xy]-/);
    expect(pending?.parentElement).toHaveAttribute("data-sidebar", "menu-item");

    workspaceState.workspaces = WORKSPACES;
    workspaceState.currentWorkspace = WORKSPACES[0];
    workspaceState.loading = false;
    rerender(
      <SidebarProvider>
        <WorkspaceSwitcher />
      </SidebarProvider>,
    );
    const trigger = screen.getByRole("button", { name: /acme-hq/ });
    expect(trigger).toHaveClass("h-8", "p-2", "gap-2");
    expect(trigger.parentElement).toHaveAttribute("data-sidebar", "menu-item");
  });

  it("shows the current workspace's name as the trigger", () => {
    renderSwitcher();
    expect(screen.getByRole("button", { name: /acme-hq/ })).toBeInTheDocument();
  });

  // w4/471: assistive technology must be able to tell which workspace is current.
  it("marks exactly the current workspace as checked", async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    const current = await screen.findByRole("menuitemradio", {
      name: /acme-hq/,
    });
    expect(current).toHaveAttribute("aria-checked", "true");
    const checked = screen
      .getAllByRole("menuitemradio")
      .filter((item) => item.getAttribute("aria-checked") === "true");
    expect(checked).toEqual([current]);
    expect(
      screen.getByRole("menuitemradio", { name: /acme-staging/ }),
    ).toHaveAttribute("aria-checked", "false");
  });

  it("lists every workspace and switches on click", async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    await user.click(
      await screen.findByRole("menuitemradio", { name: /acme-staging/ }),
    );

    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-2");
  });

  // w9/m620: billing details are Admin, Developer and Billing, as on Render;
  // a Contributor's or a Viewer's switcher offers no Billing entry once the
  // capability read confirms the role.
  it("offers no Billing entry to a role that may not view billing details", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canViewBilling: false }),
    );
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    expect(
      await screen.findByRole("menuitem", { name: "Workspace Settings" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Billing" }),
    ).not.toBeInTheDocument();
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
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

  // w4/m165: switching from a page about one of the old workspace's resources
  // must leave it, or the new workspace's chrome frames the old one's data.
  it.each([
    ["project", { projectId: "prj-1" }],
    ["service", { serviceId: "srv-1" }],
  ])(
    "lands on the new workspace's overview when switching from a %s page",
    async (_kind, params) => {
      routeParams = params;
      const user = userEvent.setup();
      renderSwitcher();

      await user.click(screen.getByRole("button", { name: /acme-hq/ }));
      await user.click(
        await screen.findByRole("menuitemradio", { name: /acme-staging/ }),
      );

      expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-2");
      expect(mockNavigate).toHaveBeenCalledWith({ to: "/" });
    },
  );

  it("stays on a workspace-level page such as billing when switching", async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    await user.click(
      await screen.findByRole("menuitemradio", { name: /acme-staging/ }),
    );

    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-2");
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  // w7/m464: Render puts the org in this dropdown ("open the workspace
  // dropdown at the top of the left pane and select your org").
  describe("organizations", () => {
    const acme = {
      id: "org-1",
      name: "Acme",
      primaryWorkspaceId: "tea-hq",
      requireTwoFactor: false,
      requireGoogleLogin: false,
    };
    const orgWorkspace = (
      id: string,
      name: string,
      primary = false,
    ): SwitcherWorkspace => ({
      id,
      name,
      plan: "scale",
      role: "admin",
      createdAt: null,
      organization: acme,
      primary,
      privacy: primary ? "invite_only" : "public",
    });
    const personal: SwitcherWorkspace = {
      id: "tea-me",
      name: "personal",
      plan: "hobby",
      role: "admin",
      createdAt: null,
      organization: null,
      primary: false,
      privacy: null,
    };
    const listed = (role: string) =>
      vi.mocked(useOrganizations).mockReturnValue({
        organizations: [organization(role as "OWNER")],
      });

    /** The menu's entries in order, with the label rows between them. */
    async function menu(trigger: RegExp) {
      const user = userEvent.setup();
      renderSwitcher();
      await user.click(screen.getByRole("button", { name: trigger }));
      await screen.findByRole("menu");
      return [
        ...document.querySelectorAll(
          "[role=menuitem], [role=menuitemradio], [data-slot=dropdown-menu-label]",
        ),
      ].map((item) => {
        const role = item.getAttribute("role") ?? "label";
        const name = item.querySelector(".truncate")?.textContent;
        return `${role}:${name ?? item.textContent}`;
      });
    }

    it("lists an Owner's org with their workspaces in it, the primary first, then the rest", async () => {
      listed("OWNER");
      const hq = orgWorkspace("tea-hq", "acme-hq", true);
      workspaceState.workspaces = [
        personal,
        orgWorkspace("tea-web", "acme-web"),
        hq,
      ];
      workspaceState.currentWorkspace = hq;

      expect(await menu(/acme-hq/)).toEqual([
        "menuitem:Billing",
        "menuitem:Workspace Settings",
        "menuitem:Acme",
        "menuitemradio:acme-hq",
        "menuitemradio:acme-web",
        "label:Workspaces",
        "menuitemradio:personal",
        "menuitem:+ New Workspace",
      ]);
      expect(
        screen.getByRole("menuitemradio", { name: /acme-hq/ }),
      ).toHaveAttribute("aria-checked", "true");
    });

    it("opens the org's Workspaces page from its entry", async () => {
      listed("OWNER");
      workspaceState.workspaces = [personal];
      workspaceState.currentWorkspace = personal;
      const user = userEvent.setup();
      renderSwitcher();

      await user.click(screen.getByRole("button", { name: /personal/ }));
      // An Owner is in none of the org's workspaces until they join one, and
      // still reaches the org.
      await user.click(
        await screen.findByRole("menuitem", { name: /Acme.*Organization/ }),
      );
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/org/$orgId/workspaces",
        params: { orgId: "org-1" },
      });
    });

    it("lists only the workspaces a Member is in under the org", async () => {
      listed("MEMBER");
      const web = orgWorkspace("tea-web", "acme-web");
      workspaceState.workspaces = [web, personal];
      workspaceState.currentWorkspace = personal;

      expect(await menu(/personal/)).toEqual([
        "menuitem:Billing",
        "menuitem:Workspace Settings",
        "menuitem:Acme",
        "menuitemradio:acme-web",
        "label:Workspaces",
        "menuitemradio:personal",
        "menuitem:+ New Workspace",
      ]);
    });

    it("shows a Guest their org's name and their one workspace", async () => {
      listed("GUEST");
      const guestWorkspace = orgWorkspace("tea-web", "acme-web");
      workspaceState.workspaces = [guestWorkspace];
      workspaceState.currentWorkspace = guestWorkspace;

      expect(await menu(/acme-web/)).toEqual([
        "menuitem:Billing",
        "menuitem:Workspace Settings",
        "menuitem:Acme",
        "menuitemradio:acme-web",
        "menuitem:+ New Workspace",
      ]);
    });

    it("groups a workspace under its own org before the org list arrives", async () => {
      const hq = orgWorkspace("tea-hq", "acme-hq", true);
      workspaceState.workspaces = [hq];
      workspaceState.currentWorkspace = hq;

      expect(await menu(/acme-hq/)).toEqual([
        "menuitem:Billing",
        "menuitem:Workspace Settings",
        "menuitem:Acme",
        "menuitemradio:acme-hq",
        "menuitem:+ New Workspace",
      ]);
    });
  });

  it("does nothing when the current workspace is picked again", async () => {
    routeParams = { serviceId: "srv-1" };
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole("button", { name: /acme-hq/ }));
    await user.click(
      await screen.findByRole("menuitemradio", { name: /acme-hq/ }),
    );

    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
