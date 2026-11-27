import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { Route as OrganizationWorkspacesRoute } from "@/routes/org.$orgId.workspaces";
import { OrganizationWorkspacesPage } from "@/features/organizations/pages/organization-workspaces-page";
import { OrganizationWorkspacesRouteSkeleton } from "@/features/organizations/components/organization-route-skeletons";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import type { UseOrganizationResult } from "@/features/organizations/hooks/use-organization";
import {
  orgWorkspace,
  organization,
  KNOWN_FROM,
  pageStructure,
  renderPendingAtOrgRoute,
  renderAtOrgRoute,
} from "@/features/organizations/__tests__/org-fixtures";
import {
  toOrganizationRole,
  type Organization,
} from "@/features/organizations/types";

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

// The org's login check reads through Apollo (w7/m466); these pages' tests
// stand for a caller the org does not refuse.
vi.mock("@/features/organizations/hooks/use-organization-login-check", () => ({
  useOrganizationLoginCheck: () => null,
}));

const setCurrentWorkspaceId = vi.fn();
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ setCurrentWorkspaceId }),
}));

let state: UseOrganizationResult;
vi.mock("@/features/organizations/hooks/use-organization", () => ({
  useOrganization: () => state,
}));

const join = vi.fn(async () => true);
const joinAsAdmin = vi.fn(async () => true);
vi.mock(
  "@/features/organizations/hooks/use-organization-workspace-actions",
  () => ({
    useJoinOrganizationWorkspace: () => ({ join, joinAsAdmin, joining: null }),
    useCreateOrganizationWorkspace: () => ({
      create: vi.fn(),
      busy: false,
      refusal: null,
      clearRefusal: vi.fn(),
    }),
  }),
);

function ready(org: Organization): UseOrganizationResult {
  return {
    organization: org,
    role: toOrganizationRole(org.viewerRole),
    loading: false,
    notFound: false,
    workspaceName: (id) => org.workspaces.find((w) => w.id === id)?.name,
  };
}

beforeEach(() => {
  setCurrentWorkspaceId.mockReset();
  join.mockClear();
  joinAsAdmin.mockClear();
  vi.mocked(useOrganizations).mockReturnValue({
    organizations: [],
  });
});

async function renderPage(org: Organization) {
  state = ready(org);
  const view = renderAtOrgRoute(
    "workspaces",
    <OrganizationWorkspacesPage organizationId="org-1" />,
  );
  await screen.findByRole("heading", { level: 1, name: "Acme" });
  return view;
}

/** A workspace's row, found by its name. */
function row(name: string) {
  const found = screen.getAllByRole("row").find(
    (tr) =>
      // The first span after the avatar (w1/m648).
      tr.querySelector("td span:not([data-slot])")?.textContent === name,
  );
  if (!found) throw new Error(`no row for ${name}`);
  return found;
}

function rowButtons(name: string) {
  return within(row(name))
    .queryAllByRole("button")
    .map((button) => button.firstChild?.textContent ?? "");
}

describe("OrganizationWorkspacesPage (w7/m464)", () => {
  it("lists every workspace with its privacy and the Primary label", async () => {
    await renderPage(organization("OWNER"));
    expect(within(row("acme-hq")).getByText("Primary")).toBeInTheDocument();
    expect(within(row("acme-hq")).getByText("Invite-only")).toBeInTheDocument();
    expect(within(row("acme-web")).getByText("Public")).toBeInTheDocument();
    expect(within(row("acme-web")).queryByText("Primary")).toBeNull();
    expect(screen.getByRole("link", { name: "Workspaces" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("gives an Owner New Workspace, and Join as admin on every workspace they are not in", async () => {
    await renderPage(organization("OWNER"));
    expect(
      screen.getByRole("button", { name: "New Workspace" }),
    ).toBeInTheDocument();
    expect(rowButtons("acme-hq")).toEqual(["Open"]);
    expect(rowButtons("acme-web")).toEqual(["Join", "Join as admin"]);
    expect(rowButtons("acme-ml")).toEqual(["Join as admin"]);

    const user = userEvent.setup();
    await user.click(
      within(row("acme-ml")).getByRole("button", {
        name: "Join as admin",
        description: "acme-ml",
      }),
    );
    expect(joinAsAdmin).toHaveBeenCalledWith("tea-ml", "acme-ml");
  });

  it("offers a Member Join on a Public workspace and nothing on an Invite-only one", async () => {
    await renderPage(organization("MEMBER"));
    expect(screen.queryByRole("button", { name: "New Workspace" })).toBeNull();
    expect(rowButtons("acme-web")).toEqual(["Join"]);
    expect(rowButtons("acme-ml")).toEqual([]);
    expect(rowButtons("acme-hq")).toEqual([]);
    expect(screen.queryByRole("button", { name: /Join as admin/ })).toBeNull();

    const user = userEvent.setup();
    await user.click(
      within(row("acme-web")).getByRole("button", {
        name: "Join",
        description: "acme-web",
      }),
    );
    expect(join).toHaveBeenCalledWith("tea-web", "acme-web");
  });

  it("shows a Guest their one workspace, to open, and no joins", async () => {
    await renderPage(
      organization("GUEST", [
        orgWorkspace({ id: "tea-web", name: "acme-web", privacy: "public" }),
      ]),
    );
    expect(screen.getAllByRole("row")).toHaveLength(2);
    expect(rowButtons("acme-web")).toEqual(["Open"]);
    expect(screen.queryByRole("button", { name: "New Workspace" })).toBeNull();

    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", { name: "Open", description: "acme-web" }),
    );
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-web");
    await screen.findByText("overview");
  });

  // w3/m705: the route's `?new=workspace`, where Render's
  // /organization/<id>/new/workspace lands, opens an Owner's New Workspace
  // dialog; closing it clears the param.
  async function renderRouteAt(href: string, org: Organization) {
    state = ready(org);
    const rootRoute = createRootRoute();
    const page = createRoute({
      getParentRoute: () => rootRoute,
      path: "/org/$orgId/workspaces",
      validateSearch: OrganizationWorkspacesRoute.options
        .validateSearch as never,
      component: OrganizationWorkspacesRoute.options.component!,
    });
    const router = createRouter({
      // The file Route's options are typed for the generated tree, not this one.
      routeTree: rootRoute.addChildren([page as never]),
      history: createMemoryHistory({ initialEntries: [href] }),
      context: { client: {} as never, session: null },
    });
    render(<RouterProvider router={router} />);
    // An open dialog hides the page from the accessibility tree.
    await screen.findByRole("heading", {
      level: 1,
      name: "Acme",
      hidden: true,
    });
    return router;
  }

  it("opens an Owner's New Workspace from ?new=workspace and clears it on close", async () => {
    const router = await renderRouteAt(
      "/org/org-1/workspaces?new=workspace",
      organization("OWNER"),
    );
    expect(
      await screen.findByRole("heading", { name: "New workspace in Acme" }),
    ).toBeInTheDocument();
    const user = userEvent.setup();
    await user.keyboard("{Escape}");
    await waitFor(() =>
      expect(router.state.location.search).not.toHaveProperty("new"),
    );
    expect(screen.queryByRole("dialog")).toBeNull();

    await user.click(screen.getByRole("button", { name: "New Workspace" }));
    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({ new: "workspace" }),
    );
    expect(router.history.length).toBe(1);
  });

  it("opens nothing for a Member, who cannot create workspaces", async () => {
    await renderRouteAt(
      "/org/org-1/workspaces?new=workspace",
      organization("MEMBER"),
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("says when the org is not found or the caller is not in it", async () => {
    state = {
      ...ready(organization("OWNER")),
      organization: null,
      role: null,
      notFound: true,
    };
    renderAtOrgRoute(
      "workspaces",
      <OrganizationWorkspacesPage organizationId="org-1" />,
    );
    expect(
      await screen.findByText("Organization not found"),
    ).toBeInTheDocument();
  });

  // The route's pending frame is the ready page's structure: header, the
  // navigation, the card with the Owner's action, the columns, and each row
  // with its badge and actions, read from the org the client already holds.
  it.each(
    (["OWNER", "MEMBER", "GUEST"] as const).flatMap((role) =>
      KNOWN_FROM.map((knownFrom) => [role, knownFrom] as const),
    ),
  )(
    "previews the ready page for an %s known from the %s",
    async (role, knownFrom) => {
      const org =
        role === "GUEST"
          ? organization("GUEST", [
              orgWorkspace({ id: "tea-web", name: "acme-web" }),
            ])
          : organization(role);
      const ready = await renderPage(org);
      const readyStructure = pageStructure(ready.container);
      ready.unmount();

      const pending = renderPendingAtOrgRoute(
        "workspaces",
        <OrganizationWorkspacesRouteSkeleton />,
        org,
        knownFrom,
      );
      await waitFor(() =>
        expect(
          pending.container.querySelector(
            '[data-route-skeleton="organization-workspaces"]',
          ),
        ).not.toBeNull(),
      );
      expect(pageStructure(pending.container)).toEqual(readyStructure);
      expect(readyStructure.rows).toHaveLength(org.workspaces.length);
    },
  );
});
