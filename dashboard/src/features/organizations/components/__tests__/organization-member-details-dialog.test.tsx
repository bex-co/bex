import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { OrganizationMemberDetailsDialog } from "@/features/organizations/components/organization-member-details-dialog";
import { loginMethodLabel } from "@/features/organizations/lib/login-methods";
import type { OrganizationMember } from "@/features/organizations/hooks/use-organization-members";
import {
  orgWorkspace,
  scriptedClient,
} from "@/features/organizations/__tests__/org-fixtures";

function member(
  overrides: Partial<OrganizationMember> = {},
): OrganizationMember {
  return {
    __typename: "OrganizationMember",
    subject: "sub-ada",
    userId: "usr-ada",
    email: "ada@example.com",
    role: "OWNER",
    createdAt: null,
    workspaces: ["tea-hq", "tea-ml"],
    workspaceRoles: [
      {
        __typename: "OrganizationMemberWorkspace",
        workspaceId: "tea-hq",
        role: "ADMIN",
      },
      {
        __typename: "OrganizationMemberWorkspace",
        workspaceId: "tea-ml",
        role: "VIEWER",
      },
    ],
    isSelf: false,
    avatarDownload: null,
    ...overrides,
  };
}

const WORKSPACES = [
  orgWorkspace({ id: "tea-hq", name: "acme-hq", privacy: "public" }),
  orgWorkspace({ id: "tea-ml", name: "acme-ml", privacy: "invite_only" }),
];

const STATUS = {
  orgUserAuthStatus: {
    __typename: "OrgUserAuthStatus",
    is2FAEnabled: false,
    loginMethodStatuses: [
      { __typename: "LoginMethodStatus", method: "pwd", signedIn: true },
      { __typename: "LoginMethodStatus", method: "google", signedIn: false },
      { __typename: "LoginMethodStatus", method: "github", signedIn: true },
    ],
  },
};

function renderDetails(
  canViewAuthStatus: boolean,
  who: OrganizationMember,
  answers: Parameters<typeof scriptedClient>[0] = {
    orgUserAuthStatus: () => STATUS,
  },
) {
  const script = scriptedClient(answers);
  render(
    <OrganizationMemberDetailsDialog
      organizationId="org-1"
      member={who}
      canViewAuthStatus={canViewAuthStatus}
      workspace={(id) => WORKSPACES.find((w) => w.id === id)}
      onClose={vi.fn()}
    />,
    { wrapper: script.wrapper },
  );
  return script;
}

// w1/m657: Render's View details on an org member.
describe("OrganizationMemberDetailsDialog", () => {
  it("shows an Owner the member's workspaces, login methods and 2FA", async () => {
    const { sent } = renderDetails(true, member());
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("ada@example.com")).toBeInTheDocument();
    expect(within(dialog).getByText("Owner")).toBeInTheDocument();

    const workspaces = within(dialog).getAllByRole("listitem").slice(0, 2);
    expect(workspaces[0]).toHaveTextContent("acme-hq");
    expect(workspaces[0]).toHaveTextContent("Admin");
    expect(workspaces[1]).toHaveTextContent("acme-ml");
    expect(workspaces[1]).toHaveTextContent("Viewer");
    expect(
      within(workspaces[1]).getByRole("img", { name: "Invite-only" }),
    ).toBeInTheDocument();
    expect(
      within(workspaces[0]).queryByRole("img", { name: "Invite-only" }),
    ).toBeNull();

    expect(
      await within(dialog).findByText("Login & Authentication Status"),
    ).toBeInTheDocument();
    const rows = (await within(dialog).findByText("Password")).closest("ul")!;
    expect(
      within(rows)
        .getAllByRole("listitem")
        .map((row) => row.textContent),
    ).toEqual([
      "PasswordSigned in",
      "GoogleNot signed in",
      "GithubSigned in",
      "Two-factor AuthenticationNot enabled",
    ]);
    expect(sent).toEqual([
      {
        name: "orgUserAuthStatus",
        variables: { userId: "usr-ada", organizationId: "org-1" },
      },
    ]);
  });

  it("shows anyone else the workspaces only, and asks nothing", () => {
    const { sent } = renderDetails(false, member({ role: "MEMBER" }), {});
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).queryByText("Owner")).toBeNull();
    expect(within(dialog).getByText("acme-hq")).toBeInTheDocument();
    expect(
      within(dialog).queryByText("Login & Authentication Status"),
    ).toBeNull();
    expect(sent).toEqual([]);
  });

  it("says when the member belongs to no workspace", () => {
    renderDetails(false, member({ workspaces: [], workspaceRoles: [] }), {});
    expect(screen.getByText("No workspaces")).toBeInTheDocument();
    expect(
      screen.getByText("This member has not been invited to a workspace."),
    ).toBeInTheDocument();
  });

  it("never shows a status it could not read", async () => {
    const { sent } = renderDetails(true, member(), {
      orgUserAuthStatus: () =>
        new GraphQLError("identity provider lookup failed"),
    });
    await vi.waitFor(() => expect(sent).toHaveLength(1));
    await vi.waitFor(() =>
      expect(screen.queryByText("Loading login status…")).toBeNull(),
    );
    expect(screen.queryByText("Not signed in")).toBeNull();
    expect(screen.queryByText("Login & Authentication Status")).toBeNull();
  });
});

describe("loginMethodLabel", () => {
  it("names methods as Render's dialog does", () => {
    expect(loginMethodLabel("pwd", "Password")).toBe("Password");
    expect(loginMethodLabel("google", "Password")).toBe("Google");
    expect(loginMethodLabel("github", "Password")).toBe("Github");
  });
});
