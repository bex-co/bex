import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloProvider } from "@apollo/client/react";
import { toast } from "sonner";
import { ApolloLink } from "@apollo/client";
import { GraphQLError } from "graphql";
import { Observable } from "rxjs";
import { WorkspacesDocument } from "@/graphql/definitions";
import { OrganizationSecurityCard } from "@/features/organizations/components/organization-security-card";
import { useOrganization } from "@/features/organizations/hooks/use-organization";
import {
  organization,
  refusal,
  renderAtOrgRoute,
  scriptedClient,
} from "@/features/organizations/__tests__/org-fixtures";
import type {
  Organization,
  OrganizationRole,
} from "@/features/organizations/types";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const startGoogleSignIn = vi.hoisted(() => vi.fn());
vi.mock("@/features/auth/lib/google-sign-in", () => ({ startGoogleSignIn }));

beforeEach(() => {
  vi.mocked(toast.success).mockReset();
  startGoogleSignIn.mockReset();
  startGoogleSignIn.mockResolvedValue(true);
});

/** The card as the Settings page renders it, from the org the client holds. */
function SecurityCard() {
  const { organization: org, role } = useOrganization("org-1");
  return org ? (
    <OrganizationSecurityCard organization={org} canChange={role === "OWNER"} />
  ) : null;
}

type Answer = Record<string, unknown> | ReturnType<typeof refusal>;

/**
 * Org Settings -> Security against a client whose network answers the org
 * (as `role` sees it, with `settings`) and each mutation by `answers`. The
 * caller's workspace list is cached too, holding one of the org's workspaces,
 * so a test can read what the change does to it.
 */
async function renderCard(
  role: OrganizationRole,
  settings: Partial<Organization> = {},
  answers: Record<string, () => Answer> = {},
) {
  const script = scriptedClient({
    Organization: () => ({
      organization: { ...organization(role), ...settings },
    }),
    // The confirmation's list (w7/m471): nobody, unless a test says.
    OrganizationLoginRequirementImpact: () => ({
      organizationLoginRequirementImpact: {
        __typename: "LoginRequirementImpact",
        members: [],
        unattributedApiKeys: [],
      },
    }),
    ...answers,
  });
  script.client.writeQuery({
    query: WorkspacesDocument,
    data: {
      workspaces: [
        {
          __typename: "Workspace",
          id: "tea-hq",
          name: "acme-hq",
          slug: "acme-hq",
          plan: "scale",
          role: "admin",
          createdAt: null,
          requireTwoFactor: false,
          requireGoogleLogin: false,
          organization: {
            __typename: "WorkspaceOrganization",
            id: "org-1",
            name: "Acme",
            primaryWorkspaceId: "tea-hq",
            requireTwoFactor: false,
            requireGoogleLogin: false,
          },
          primary: true,
          privacy: "invite_only",
        },
      ],
    },
  });
  renderAtOrgRoute(
    "settings",
    <ApolloProvider client={script.client}>
      <SecurityCard />
    </ApolloProvider>,
  );
  await screen.findByRole("heading", { name: "Security" });
  return { ...script, user: userEvent.setup() };
}

/** bex-api's answer to an accepted change: the whole org, as written. */
function accepted(field: string, settings: Partial<Organization>) {
  return () => ({ [field]: { ...organization("OWNER"), ...settings } });
}

function sentVariables(
  sent: { name: string; variables: Record<string, unknown> }[],
  name: string,
) {
  return sent.filter((op) => op.name === name).map((op) => op.variables);
}

describe("OrganizationSecurityCard — an Owner (w7/m466)", () => {
  it("requires two-factor authentication after confirming, and every cached copy of the org follows", async () => {
    const { user, sent, client } = await renderCard(
      "OWNER",
      {},
      {
        SetOrganizationRequireTwoFactor: accepted(
          "setOrganizationRequireTwoFactor",
          { requireTwoFactor: true },
        ),
      },
    );
    const toggle = screen.getByRole("switch", {
      name: "Require two-factor authentication",
    });
    expect(toggle).not.toBeChecked();

    await user.click(toggle);
    const dialog = await screen.findByRole("alertdialog", {
      name: "Require two-factor authentication in Acme?",
    });
    expect(dialog).toHaveTextContent(
      "Each workspace's own setting stays as it is.",
    );
    expect(sentVariables(sent, "SetOrganizationRequireTwoFactor")).toEqual([]);
    await user.click(within(dialog).getByRole("button", { name: "Require" }));

    await waitFor(() => expect(toggle).toBeChecked());
    expect(sentVariables(sent, "SetOrganizationRequireTwoFactor")).toEqual([
      { organizationId: "org-1", enabled: true },
    ]);
    expect(sentVariables(sent, "SetOrganizationRequireGoogleLogin")).toEqual(
      [],
    );
    expect(toast.success).toHaveBeenCalledWith(
      "Two-factor authentication is now required in every workspace of Acme",
    );
    // The workspace list carries the org's setting on each workspace's org,
    // where it locks the workspace's own switch: it follows without a refetch.
    const cached = client.readQuery({ query: WorkspacesDocument });
    expect(cached?.workspaces?.[0]?.organization).toMatchObject({
      requireTwoFactor: true,
      requireGoogleLogin: false,
    });
  });

  it("stops requiring Google login at once, with no confirmation", async () => {
    const { user, sent } = await renderCard(
      "OWNER",
      { requireGoogleLogin: true },
      {
        SetOrganizationRequireGoogleLogin: accepted(
          "setOrganizationRequireGoogleLogin",
          { requireGoogleLogin: false },
        ),
      },
    );
    const toggle = screen.getByRole("switch", { name: "Require Google login" });
    expect(toggle).toBeChecked();

    await user.click(toggle);

    await waitFor(() => expect(toggle).not.toBeChecked());
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(sentVariables(sent, "SetOrganizationRequireGoogleLogin")).toEqual([
      { organizationId: "org-1", enabled: false },
    ]);
    expect(toast.success).toHaveBeenCalledWith(
      "Acme no longer requires Google login",
    );
  });

  // The lock-out guard: an Owner who does not meet a requirement cannot turn
  // it on, and is told how to meet it on their own account.
  it("says why it cannot require 2FA of an Owner without a second factor, and how to fix it", async () => {
    const { user } = await renderCard(
      "OWNER",
      {},
      {
        SetOrganizationRequireTwoFactor: () =>
          refusal("WORKSPACE_2FA_ADMIN_NOT_ENROLLED", {
            organizationId: "org-1",
          }),
      },
    );
    const toggle = screen.getByRole("switch", {
      name: "Require two-factor authentication",
    });
    await user.click(toggle);
    await user.click(await screen.findByRole("button", { name: "Require" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Couldn't change the two-factor setting");
    expect(alert).toHaveTextContent(
      "Turn on two-factor authentication for your own account before requiring it in this organization, or you would lock yourself out.",
    );
    expect(alert).not.toHaveTextContent("reworded by the server");
    expect(
      within(alert).getByRole("link", {
        name: "Set up two-factor authentication",
      }),
    ).toHaveAttribute("href", "/settings#security");
    expect(toggle).not.toBeChecked();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("says why it cannot require Google login of an Owner not signed in with Google, and signs them in", async () => {
    const { user } = await renderCard(
      "OWNER",
      {},
      {
        SetOrganizationRequireGoogleLogin: () =>
          refusal("WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN", {
            organizationId: "org-1",
          }),
      },
    );
    await user.click(
      screen.getByRole("switch", { name: "Require Google login" }),
    );
    await user.click(await screen.findByRole("button", { name: "Require" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Couldn't change the Google login setting");
    expect(alert).toHaveTextContent(
      "Sign in with the Google account of your bex email address before requiring Google login in this organization, or you would lock yourself out.",
    );
    await user.click(
      within(alert).getByRole("button", { name: "Sign in with Google" }),
    );
    expect(startGoogleSignIn).toHaveBeenCalledTimes(1);
  });

  it("says when the installation cannot enforce it, with nothing to fix", async () => {
    const { user } = await renderCard(
      "OWNER",
      {},
      {
        SetOrganizationRequireGoogleLogin: () =>
          refusal("WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE"),
      },
    );
    await user.click(
      screen.getByRole("switch", { name: "Require Google login" }),
    );
    await user.click(await screen.findByRole("button", { name: "Require" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      "Google login enforcement is unavailable on this installation, so it can't be turned on.",
    );
    expect(within(alert).queryByRole("button")).toBeNull();
    expect(within(alert).queryByRole("link")).toBeNull();
  });

  // Any other refusal is worded as every org action's: a caller who is no
  // longer an Owner is refused FORBIDDEN.
  it("words a refusal that is not the setting's own as the org's actions do", async () => {
    const { user } = await renderCard(
      "OWNER",
      { requireTwoFactor: true },
      { SetOrganizationRequireTwoFactor: () => refusal("FORBIDDEN") },
    );
    await user.click(
      screen.getByRole("switch", { name: "Require two-factor authentication" }),
    );

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      "You don't have permission to do that in this organization.",
    );
    expect(alert).not.toHaveTextContent("reworded by the server");
  });
});

describe("OrganizationSecurityCard — a Member (w7/m466)", () => {
  it("shows each setting's state without a switch", async () => {
    const { sent } = await renderCard("MEMBER", { requireGoogleLogin: true });

    expect(screen.queryByRole("switch")).toBeNull();
    expect(
      [...document.querySelectorAll("[data-login-requirement]")].map(
        (row) => row.querySelector("[data-slot=badge]")?.textContent,
      ),
    ).toEqual(["Not required", "Required"]);
    expect(
      screen.getByText("Only Owners can change these settings."),
    ).toBeInTheDocument();
    expect(sent.map((op) => op.name)).toEqual(["Organization"]);
  });
});

// The confirmation's list (w7/m471, docs/ADR012-auth.md 9c): who the org-wide
// requirement would block in any org workspace, from bex-api's
// organizationLoginRequirementImpact, each API key named with its workspace.
describe("OrganizationSecurityCard — who it would block (w7/m471)", () => {
  function impact(
    members: Record<string, unknown>[],
    unattributedApiKeys: Record<string, unknown>[] = [],
  ) {
    return () => ({
      organizationLoginRequirementImpact: {
        __typename: "LoginRequirementImpact",
        members,
        unattributedApiKeys,
      },
    });
  }

  async function openConfirm(
    answer: () => Record<string, unknown> | GraphQLError,
  ) {
    const rendered = await renderCard(
      "OWNER",
      {},
      {
        OrganizationLoginRequirementImpact: answer,
      },
    );
    await rendered.user.click(
      screen.getByRole("switch", { name: "Require two-factor authentication" }),
    );
    const dialog = await screen.findByRole("alertdialog", {
      name: "Require two-factor authentication in Acme?",
    });
    return { ...rendered, dialog };
  }

  it("lists a Guest with their key and its workspace, and keeps the org's sentence", async () => {
    const { dialog, sent } = await openConfirm(
      impact(
        [
          {
            __typename: "LoginRequirementImpactMember",
            userId: "own-guest",
            email: "guest@acme.test",
            name: "Guest",
            losesAccess: true,
            apiKeys: [
              {
                __typename: "LoginRequirementImpactApiKey",
                id: "key-guest",
                name: "guest key",
                workspaceName: "acme-ml",
              },
            ],
            sshKeys: [],
          },
        ],
        [
          {
            __typename: "LoginRequirementImpactApiKey",
            id: "key-old",
            name: "legacy",
            workspaceName: "acme-hq",
          },
        ],
      ),
    );

    const list = await within(dialog).findByText("guest@acme.test");
    const member = list.closest("li")!;
    expect(member).toHaveTextContent("Guest");
    expect(member).toHaveTextContent("Loses access");
    expect(member).toHaveTextContent("API key guest key in acme-ml");
    expect(dialog).toHaveTextContent("API key legacy in acme-hq");
    expect(dialog).toHaveTextContent(
      "Each workspace's own setting stays as it is.",
    );
    expect(sentVariables(sent, "OrganizationLoginRequirementImpact")).toEqual([
      { organizationId: "org-1", requirement: "TWO_FACTOR" },
    ]);
  });

  it("keeps confirm open while it checks, in the list's own box", async () => {
    // A link ahead of the network that never answers the list.
    const hold = new ApolloLink((operation, forward) =>
      operation.operationName === "OrganizationLoginRequirementImpact"
        ? new Observable(() => {})
        : forward(operation),
    );
    const script = scriptedClient(
      {
        Organization: () => ({ organization: organization("OWNER") }),
      },
      [hold],
    );
    renderAtOrgRoute(
      "settings",
      <ApolloProvider client={script.client}>
        <SecurityCard />
      </ApolloProvider>,
    );
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole("switch", {
        name: "Require two-factor authentication",
      }),
    );
    const dialog = await screen.findByRole("alertdialog");
    const list = within(dialog).getByRole("region", {
      name: "Who would lose access",
    });
    expect(list).toHaveAttribute("aria-busy", "true");
    expect(list.className).toContain("h-48");
    expect(
      within(dialog).getByRole("button", { name: "Require" }),
    ).toBeEnabled();
  });

  it("says nobody would be blocked when every person meets it", async () => {
    const { dialog } = await openConfirm(impact([]));

    expect(
      await within(dialog).findByText(
        "Nobody would lose access: every member meets this requirement, and no API key or SSH key would stop working.",
      ),
    ).toBeInTheDocument();
  });

  it("says it could not check, and the Owner can still confirm", async () => {
    const { dialog, user, sent } = await openConfirm(
      () => new GraphQLError("authorization service unavailable"),
    );

    expect(
      await within(dialog).findByText(
        "Couldn't check who would lose access. You can still turn it on.",
      ),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Require" }));
    await waitFor(() =>
      expect(sentVariables(sent, "SetOrganizationRequireTwoFactor")).toEqual([
        { organizationId: "org-1", enabled: true },
      ]),
    );
  });
});
