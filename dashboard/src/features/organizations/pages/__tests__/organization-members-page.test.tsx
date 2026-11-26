import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OrganizationMembersPage } from "@/features/organizations/pages/organization-members-page";
import { OrganizationMembersRouteSkeleton } from "@/features/organizations/components/organization-route-skeletons";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import type { UseOrganizationResult } from "@/features/organizations/hooks/use-organization";
import type {
  OrganizationInvite,
  OrganizationMember,
  UseOrganizationMembersResult,
} from "@/features/organizations/hooks/use-organization-members";
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
  type OrganizationRole,
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

let orgState: UseOrganizationResult;
vi.mock("@/features/organizations/hooks/use-organization", () => ({
  useOrganization: () => orgState,
}));

let membersState: UseOrganizationMembersResult;
const membersHook = vi.fn();
vi.mock("@/features/organizations/hooks/use-organization-members", () => ({
  useOrganizationMembers: (...args: unknown[]) => {
    membersHook(...args);
    return membersState;
  },
}));

const changeRole = vi.fn(async () => true);
const remove = vi.fn(async () => true);
const revokeInvite = vi.fn(async () => true);
vi.mock(
  "@/features/organizations/hooks/use-organization-member-actions",
  () => ({
    useOrganizationMemberActions: () => ({
      changeRole,
      remove,
      revokeInvite,
      busy: null,
    }),
    useInviteOrganizationMember: () => ({
      invite: vi.fn(),
      busy: false,
      refusal: null,
      clearRefusal: vi.fn(),
    }),
  }),
);

const member = (
  subject: string,
  email: string,
  role: OrganizationRole,
  workspaces: string[] = [],
  isSelf = false,
): OrganizationMember => ({
  __typename: "OrganizationMember",
  subject,
  userId: `usr-${subject}`,
  email,
  role,
  createdAt: null,
  workspaces,
  workspaceRoles: workspaces.map((workspaceId) => ({
    __typename: "OrganizationMemberWorkspace" as const,
    workspaceId,
    role: "DEVELOPER",
  })),
  isSelf,
  avatarDownload: null,
});

const PEOPLE = (selfRole: OrganizationRole) => [
  member("sub-me", "me@example.com", selfRole, ["tea-hq"], true),
  member("sub-ada", "ada@example.com", "OWNER", ["tea-hq"]),
  member("sub-bob", "bob@example.com", "MEMBER", ["tea-web", "tea-ml"]),
  member("sub-gus", "gus@example.com", "GUEST", ["tea-web"]),
];

const INVITE: OrganizationInvite = {
  __typename: "OrganizationInvite",
  id: "inv-1",
  email: "new@example.com",
  role: "OWNER",
  expiresAt: null,
  createdAt: null,
};

function members(
  overrides: Partial<UseOrganizationMembersResult> = {},
): UseOrganizationMembersResult {
  return {
    members: [],
    invites: [],
    loading: false,
    forbidden: false,
    failed: false,
    ...overrides,
  };
}

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
  changeRole.mockClear();
  remove.mockClear();
  revokeInvite.mockClear();
  membersHook.mockClear();
  vi.mocked(useOrganizations).mockReturnValue({
    organizations: [],
  });
});

async function renderPage(role: OrganizationRole) {
  orgState = ready(organization(role));
  const view = renderAtOrgRoute(
    "members",
    <OrganizationMembersPage organizationId="org-1" />,
  );
  await screen.findByRole("heading", { level: 1, name: "Acme" });
  return view;
}

function row(email: string) {
  const found = screen.getAllByRole("row").find(
    (tr) =>
      // The first span after the avatar (w1/m648).
      tr.querySelector("td span:not([data-slot])")?.textContent === email,
  );
  if (!found) throw new Error(`no row for ${email}`);
  return found;
}

describe("OrganizationMembersPage (w7/m464)", () => {
  it("shows an Owner every role, a Guest's workspace, and the controls on everyone but themselves", async () => {
    membersState = members({ members: PEOPLE("OWNER"), invites: [INVITE] });
    await renderPage("OWNER");
    expect(membersHook).toHaveBeenCalledWith("org-1", true);

    const user = userEvent.setup();
    const self = row("me@example.com");
    expect(within(self).getByText("You")).toBeInTheDocument();
    expect(within(self).getByText("Owner")).toBeInTheDocument();
    expect(within(self).queryByRole("combobox")).toBeNull();
    // Their own row's menu only opens their details.
    await user.click(
      within(self).getByRole("button", { name: "Actions for me@example.com" }),
    );
    expect(
      screen.getAllByRole("menuitem").map((item) => item.textContent),
    ).toEqual(["View details"]);
    await user.keyboard("{Escape}");

    for (const [email, role] of [
      ["ada@example.com", "Owner"],
      ["bob@example.com", "Member"],
      ["gus@example.com", "Guest"],
    ]) {
      expect(
        within(row(email)).getByRole("combobox", {
          name: `Organization role for ${email}`,
        }),
      ).toHaveTextContent(role);
      await user.click(
        within(row(email)).getByRole("button", {
          name: `Actions for ${email}`,
        }),
      );
      expect(
        screen.getAllByRole("menuitem").map((item) => item.textContent),
      ).toEqual(["View details", "Remove member"]);
      await user.keyboard("{Escape}");
    }
    // A Guest's one workspace, by name; nobody else's list is shown.
    expect(
      within(row("gus@example.com")).getByText("Workspace: acme-web"),
    ).toBeInTheDocument();
    expect(within(row("bob@example.com")).queryByText(/Workspace:/)).toBeNull();

    expect(screen.getByRole("button", { name: "Invite" })).toBeInTheDocument();
    expect(screen.getByText("Pending invites")).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", {
        name: "Revoke",
        description: "new@example.com",
      }),
    );
    expect(revokeInvite).toHaveBeenCalledWith("inv-1", "new@example.com");
  });

  it("removes someone after the confirmation", async () => {
    membersState = members({ members: PEOPLE("OWNER") });
    await renderPage("OWNER");
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", { name: "Actions for bob@example.com" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Remove member" }));
    expect(
      await screen.findByRole("alertdialog", {
        name: "Remove from the organization?",
      }),
    ).toHaveTextContent(
      "bob@example.com loses access to the organization and to every one of its workspaces",
    );
    await user.click(screen.getByRole("button", { name: "Remove" }));
    expect(remove).toHaveBeenCalledWith("sub-bob", "bob@example.com");
  });

  it("shows a Member the roles read-only, without invites or controls", async () => {
    membersState = members({ members: PEOPLE("MEMBER") });
    await renderPage("MEMBER");
    expect(membersHook).toHaveBeenCalledWith("org-1", false);
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.queryByRole("button", { name: /Remove/ })).toBeNull();
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", { name: "Actions for ada@example.com" }),
    );
    expect(
      screen.getAllByRole("menuitem").map((item) => item.textContent),
    ).toEqual(["View details"]);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("button", { name: "Invite" })).toBeNull();
    expect(
      within(row("ada@example.com")).getByText("Owner"),
    ).toBeInTheDocument();
    expect(
      within(row("gus@example.com")).getByText("Guest"),
    ).toBeInTheDocument();
  });

  it("tells a Guest that only Owners and Members see the members", async () => {
    membersState = members({ forbidden: true });
    orgState = ready(
      organization("GUEST", [
        orgWorkspace({ id: "tea-web", name: "acme-web" }),
      ]),
    );
    renderAtOrgRoute(
      "members",
      <OrganizationMembersPage organizationId="org-1" />,
    );
    expect(
      await screen.findByText("Members are hidden from Guests"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Only Owners and Members of the organization can see who belongs to it.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("table")).toBeNull();
  });

  // The route's pending frame is the ready page's structure: the header, the
  // navigation, the card and its Owner action, and the caller's own row (or
  // a Guest's refusal), read from the org the client already holds (w4/706).
  it.each(
    (["OWNER", "MEMBER", "GUEST"] as const).flatMap((role) =>
      KNOWN_FROM.map((knownFrom) => [role, knownFrom] as const),
    ),
  )(
    "previews the ready page for an %s known from the %s",
    async (role, knownFrom) => {
      membersState =
        role === "GUEST"
          ? members({ forbidden: true })
          : members({ members: [PEOPLE(role)[0]] });
      const ready = await renderPage(role);
      const readyStructure = pageStructure(ready.container);
      const readyText = ready.container.querySelector(
        "[data-slot=card-content]",
      )?.textContent;
      ready.unmount();

      const pending = renderPendingAtOrgRoute(
        "members",
        <OrganizationMembersRouteSkeleton />,
        organization(role),
        knownFrom,
      );
      await waitFor(() =>
        expect(
          pending.container.querySelector(
            '[data-route-skeleton="organization-members"]',
          ),
        ).not.toBeNull(),
      );
      expect(pageStructure(pending.container)).toEqual(readyStructure);
      if (role === "GUEST") {
        expect(
          pending.container.querySelector("[data-slot=card-content]")
            ?.textContent,
        ).toBe(readyText);
      }
    },
  );
});
