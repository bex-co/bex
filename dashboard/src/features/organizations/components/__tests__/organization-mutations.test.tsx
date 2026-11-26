import { describe, expect, it, vi, beforeEach } from "vitest";
import {
  act,
  render,
  renderHook,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  OrganizationDocument,
  OrganizationsDocument,
  WorkspacesDocument,
} from "@/graphql/definitions";
import { CreateOrganizationWorkspaceDialog } from "@/features/organizations/components/create-organization-workspace-dialog";
import { InviteOrganizationMemberDialog } from "@/features/organizations/components/invite-organization-member-dialog";
import { OrganizationMembersCard } from "@/features/organizations/components/organization-members-card";
import { useJoinOrganizationWorkspace } from "@/features/organizations/hooks/use-organization-workspace-actions";
import {
  organization,
  refusal,
  scriptedClient,
} from "@/features/organizations/__tests__/org-fixtures";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

beforeEach(() => {
  vi.mocked(toast.success).mockReset();
  vi.mocked(toast.error).mockReset();
});

const created = (name: string, privacy: string) => ({
  createOrganizationWorkspace: {
    __typename: "Workspace",
    id: "tea-new",
    name,
    slug: name,
    plan: "scale",
    privacy,
  },
});

function sentVariables(
  sent: { name: string; variables: Record<string, unknown> }[],
  name: string,
) {
  return sent.filter((op) => op.name === name).map((op) => op.variables);
}

// w7/m464 t009: an Owner creates a workspace in the org with a name and
// Render's privacy choice, Invite-only selected; no plan, no price.
describe("CreateOrganizationWorkspaceDialog", () => {
  async function open(answer = created("acme-ml", "invite_only")) {
    const script = scriptedClient({
      CreateOrganizationWorkspace: () => answer,
    });
    const user = userEvent.setup();
    render(
      <CreateOrganizationWorkspaceDialog
        organizationId="org-1"
        organizationName="Acme"
        plan="scale"
      />,
      { wrapper: script.wrapper },
    );
    await user.click(screen.getByRole("button", { name: "New Workspace" }));
    return { user, sent: script.sent };
  }

  it("selects Invite-only and names the org's plan instead of a price", async () => {
    await open();
    expect(
      screen.getByRole("heading", { name: "New workspace in Acme" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /^Invite-only/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByRole("radio", { name: /^Public/ })).toHaveAttribute(
      "aria-checked",
      "false",
    );
    const plan = screen.getByTestId("organization-plan-included");
    expect(plan).toHaveTextContent("Included in your organization's plan");
    expect(plan).toHaveTextContent("Scale plan, at no additional cost");
    expect(screen.getByRole("dialog")).not.toHaveTextContent("$");
  });

  it("creates an Invite-only workspace by default", async () => {
    const { user, sent } = await open();
    await user.type(screen.getByLabelText("Name"), "  acme-ml ");
    await user.click(screen.getByRole("button", { name: "Create Workspace" }));

    await waitFor(() =>
      expect(sentVariables(sent, "CreateOrganizationWorkspace")).toEqual([
        { organizationId: "org-1", name: "acme-ml", privacy: "invite_only" },
      ]),
    );
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("Created acme-ml."),
    );
  });

  it("creates a Public workspace when Public is chosen", async () => {
    const { user, sent } = await open(created("acme-web", "public"));
    await user.type(screen.getByLabelText("Name"), "acme-web");
    await user.click(screen.getByRole("radio", { name: /^Public/ }));
    await user.click(screen.getByRole("button", { name: "Create Workspace" }));

    await waitFor(() =>
      expect(sentVariables(sent, "CreateOrganizationWorkspace")).toEqual([
        { organizationId: "org-1", name: "acme-web", privacy: "public" },
      ]),
    );
  });

  it("shows bex-api's refusal in the user's words", async () => {
    const { user } = await open(refusal("FORBIDDEN") as never);
    await user.type(screen.getByLabelText("Name"), "acme-ml");
    await user.click(screen.getByRole("button", { name: "Create Workspace" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "You don't have permission to do that in this organization.",
    );
  });
});

// w7/m464 t010: an Owner invites someone to the org as Member or Owner.
describe("InviteOrganizationMemberDialog", () => {
  const invite = (variables: Record<string, unknown>) => ({
    inviteOrganizationMember: {
      __typename: "OrganizationInvite",
      id: "inv-1",
      email: variables.email,
      role: variables.role,
      expiresAt: null,
      createdAt: null,
    },
  });

  async function open(answer: typeof invite | (() => unknown) = invite) {
    const script = scriptedClient({
      InviteOrganizationMember: answer as never,
    });
    const user = userEvent.setup();
    render(<InviteOrganizationMemberDialog organizationId="org-1" />, {
      wrapper: script.wrapper,
    });
    await user.click(screen.getByRole("button", { name: "Invite" }));
    return { user, sent: script.sent };
  }

  it("invites as a Member by default", async () => {
    const { user, sent } = await open();
    await user.type(screen.getByLabelText("Email address"), "ada@example.com");
    await user.click(screen.getByRole("button", { name: "Send invite" }));

    await waitFor(() =>
      expect(sentVariables(sent, "InviteOrganizationMember")).toEqual([
        { organizationId: "org-1", email: "ada@example.com", role: "MEMBER" },
      ]),
    );
  });

  it("invites as an Owner, and offers no Guest invite", async () => {
    const { user, sent } = await open();
    await user.type(screen.getByLabelText("Email address"), "ada@example.com");
    await user.click(screen.getByRole("combobox", { name: "Role" }));
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["Owner", "Member"]);
    await user.click(screen.getByRole("option", { name: "Owner" }));
    await user.click(screen.getByRole("button", { name: "Send invite" }));

    await waitFor(() =>
      expect(sentVariables(sent, "InviteOrganizationMember")).toEqual([
        { organizationId: "org-1", email: "ada@example.com", role: "OWNER" },
      ]),
    );
  });

  it("says when the person is already in the org", async () => {
    const { user } = await open(() => refusal("ORGANIZATION_ALREADY_MEMBER"));
    await user.type(screen.getByLabelText("Email address"), "ada@example.com");
    await user.click(screen.getByRole("button", { name: "Send invite" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "That person is already in the organization.",
    );
  });
});

const MEMBERS = {
  organizationMembers: [
    {
      __typename: "OrganizationMember",
      subject: "sub-me",
      userId: "usr-me",
      email: "owner@example.com",
      role: "OWNER",
      createdAt: null,
      workspaces: ["tea-hq"],
      workspaceRoles: [
        {
          __typename: "OrganizationMemberWorkspace",
          workspaceId: "tea-hq",
          role: "ADMIN",
        },
      ],
      isSelf: true,
    },
    {
      __typename: "OrganizationMember",
      subject: "sub-bob",
      userId: "usr-bob",
      email: "bob@example.com",
      role: "MEMBER",
      createdAt: null,
      workspaces: ["tea-web"],
      workspaceRoles: [
        {
          __typename: "OrganizationMemberWorkspace",
          workspaceId: "tea-web",
          role: "DEVELOPER",
        },
      ],
      isSelf: false,
    },
  ],
};

// w7/m464 t010: an Owner changes anyone else's org role and removes people;
// the API's refusals arrive in the user's words.
describe("OrganizationMembersCard (Owner)", () => {
  function renderCard(answers: Parameters<typeof scriptedClient>[0]) {
    const script = scriptedClient({
      OrganizationMembers: () => MEMBERS,
      OrganizationInvites: () => ({ organizationInvites: [] }),
      ...answers,
    });
    render(
      <OrganizationMembersCard
        organizationId="org-1"
        role="OWNER"
        workspaceName={(id) => (id === "tea-web" ? "acme-web" : undefined)}
        workspace={() => undefined}
      />,
      { wrapper: script.wrapper },
    );
    return { user: userEvent.setup(), sent: script.sent };
  }

  it("changes a member's org role to Guest", async () => {
    const { user, sent } = renderCard({
      ChangeOrganizationMemberRole: (variables) => ({
        changeOrganizationMemberRole: {
          __typename: "OrganizationMember",
          subject: variables.subject,
          role: variables.role,
          workspaces: ["tea-web"],
        },
      }),
    });
    await user.click(
      await screen.findByRole("combobox", {
        name: "Organization role for bob@example.com",
      }),
    );
    await user.click(screen.getByRole("option", { name: "Guest" }));

    await waitFor(() =>
      expect(sentVariables(sent, "ChangeOrganizationMemberRole")).toEqual([
        { organizationId: "org-1", subject: "sub-bob", role: "GUEST" },
      ]),
    );
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith(
        "bob@example.com is now Guest.",
      ),
    );
  });

  it("words the last-Owner refusal", async () => {
    const { user } = renderCard({
      ChangeOrganizationMemberRole: () => refusal("ORGANIZATION_LAST_OWNER"),
    });
    await user.click(
      await screen.findByRole("combobox", {
        name: "Organization role for bob@example.com",
      }),
    );
    await user.click(screen.getByRole("option", { name: "Guest" }));

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "An organization needs at least one Owner. Make someone else an Owner first.",
      ),
    );
  });

  it("names the workspace a removal would leave without an admin", async () => {
    const { user, sent } = renderCard({
      RemoveOrganizationMember: () =>
        refusal("ORGANIZATION_MEMBER_LAST_ADMIN", {
          organizationId: "org-1",
          workspaceId: "tea-web",
        }),
    });
    await user.click(
      await screen.findByRole("button", {
        name: "Actions for bob@example.com",
      }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Remove member" }));
    await user.click(screen.getByRole("button", { name: "Remove" }));

    await waitFor(() =>
      expect(sentVariables(sent, "RemoveOrganizationMember")).toEqual([
        { organizationId: "org-1", subject: "sub-bob" },
      ]),
    );
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "They're the last admin of acme-web. Make someone else its admin first.",
      ),
    );
  });
});

// w7/m464 t009: after a join the caller's workspace list (the switcher and
// every workspace-scoped page) and the org page are read again; the org
// page's answer updates the switcher's cached entry for the org too.
describe("useJoinOrganizationWorkspace", () => {
  it("joins, then refetches Workspaces and the Organization the switcher's list shares", async () => {
    let joined = false;
    const acme = () => {
      const org = organization("MEMBER");
      org.workspaces[1].viewerIsMember = joined;
      return org;
    };
    const script = scriptedClient({
      Workspaces: () => ({ workspaces: [] }),
      Organizations: () => ({ organizations: [acme()] }),
      Organization: () => ({ organization: acme() }),
      JoinWorkspace: () => {
        joined = true;
        return {
          joinWorkspace: {
            __typename: "JoinedWorkspace",
            workspaceId: "tea-web",
            role: "DEVELOPER",
            joined: true,
          },
        };
      },
    });
    const { result } = renderHook(
      () => {
        useQuery(WorkspacesDocument);
        const list = useQuery(OrganizationsDocument);
        useQuery(OrganizationDocument, { variables: { id: "org-1" } });
        return { list: list.data, ...useJoinOrganizationWorkspace() };
      },
      { wrapper: script.wrapper },
    );
    await waitFor(() => expect(script.sent).toHaveLength(3));

    await act(async () => {
      expect(await result.current.join("tea-web", "acme-web")).toBe(true);
    });
    expect(script.sent[3].name).toBe("JoinWorkspace");
    expect(
      script.sent
        .slice(4)
        .map((op) => op.name)
        .sort(),
    ).toEqual(["Organization", "Workspaces"]);
    expect(sentVariables(script.sent, "JoinWorkspace")).toEqual([
      { workspaceId: "tea-web" },
    ]);
    expect(
      result.current.list?.organizations[0]?.workspaces.find(
        (workspace) => workspace.id === "tea-web",
      )?.viewerIsMember,
    ).toBe(true);
    expect(toast.success).toHaveBeenCalledWith("You joined acme-web.");
  });
});
