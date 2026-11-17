import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OrganizationWorkspacesPage } from "@/features/organizations/pages/organization-workspaces-page";
import { OrganizationWorkspacesRouteSkeleton } from "@/features/organizations/components/organization-route-skeletons";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import type { UseOrganizationResult } from "@/features/organizations/hooks/use-organization";
import {
  orgWorkspace,
  organization,
  pageStructure,
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
  const found = screen
    .getAllByRole("row")
    .find((tr) => tr.querySelector("td span")?.textContent === name);
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
  // with its badge and actions, read from the switcher's cached org.
  it.each(["OWNER", "MEMBER", "GUEST"] as const)(
    "previews the ready page for an %s",
    async (role) => {
      const org =
        role === "GUEST"
          ? organization("GUEST", [
              orgWorkspace({ id: "tea-web", name: "acme-web" }),
            ])
          : organization(role);
      const ready = await renderPage(org);
      const readyStructure = pageStructure(ready.container);
      ready.unmount();

      vi.mocked(useOrganizations).mockReturnValue({
        organizations: [org],
      });
      const pending = renderAtOrgRoute(
        "workspaces",
        <OrganizationWorkspacesRouteSkeleton />,
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
