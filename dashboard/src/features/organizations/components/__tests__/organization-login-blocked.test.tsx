import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloProvider } from "@apollo/client/react";
import { createAuthErrorLink } from "@/common/apollo/auth-error-link";
import {
  clearLoginBlocks,
  reportLoginRefusal,
} from "@/features/workspaces/lib/workspace-login-block";
import { OrganizationSettingsPage } from "@/features/organizations/pages/organization-settings-page";
import { OrganizationMembersPage } from "@/features/organizations/pages/organization-members-page";
import { OrganizationSettingsRouteSkeleton } from "@/features/organizations/components/organization-route-skeletons";
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

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

const startGoogleSignIn = vi.hoisted(() => vi.fn());
vi.mock("@/features/auth/lib/google-sign-in", () => ({ startGoogleSignIn }));

beforeEach(() => {
  clearLoginBlocks();
  startGoogleSignIn.mockReset();
  startGoogleSignIn.mockResolvedValue(true);
});

afterEach(() => {
  clearLoginBlocks();
});

/** The coded 403 an org's own setting answers on the org's own objects:
 *  the org's id and name, and no workspace (w7/m466). */
function orgRefusal(code: string) {
  return refusal(code, { organizationId: "org-1", organizationName: "Acme" });
}

const member = {
  __typename: "OrganizationMember",
  subject: "kratos-1",
  userId: "usr-1",
  email: "ada@example.com",
  role: "OWNER",
  createdAt: null,
  workspaces: ["tea-hq"],
  isSelf: true,
};

/**
 * An org page against a client wired as the app's is: the auth error link
 * ahead of the network records every login refusal, and the network answers
 * the org (as `role` sees it, with `settings`) and its member list by
 * `members`.
 */
async function renderOrgPage(
  page: "settings" | "members",
  {
    role = "OWNER",
    settings = {},
    members,
  }: {
    role?: OrganizationRole;
    settings?: Partial<Organization>;
    members: () => Record<string, unknown> | ReturnType<typeof refusal>;
  },
) {
  const script = scriptedClient(
    {
      Organization: () => ({
        organization: { ...organization(role), ...settings },
      }),
      OrganizationMembers: members,
      OrganizationInvites: () => ({ organizationInvites: [] }),
    },
    [createAuthErrorLink(vi.fn(), vi.fn(), reportLoginRefusal)],
  );
  renderAtOrgRoute(
    page,
    <ApolloProvider client={script.client}>
      {page === "settings" ? (
        <OrganizationSettingsPage organizationId="org-1" />
      ) : (
        <OrganizationMembersPage organizationId="org-1" />
      )}
    </ApolloProvider>,
  );
  await screen.findByRole("heading", { level: 1, name: "Acme" });
  return { ...script, user: userEvent.setup() };
}

const TWO_FACTOR_HEADING =
  "Acme requires two-factor authentication in all of its workspaces";

describe("an org page the org's login setting refuses (w7/m466)", () => {
  // Settings reads only the org, which the server exempts so a blocked
  // person can see why; the page asks with the member list, which it holds
  // to the requirement.
  it("walls Settings behind one card naming the org, under the org's header", async () => {
    const { sent } = await renderOrgPage("settings", {
      settings: { requireTwoFactor: true },
      members: () => orgRefusal("WORKSPACE_2FA_REQUIRED"),
    });

    const card = await screen.findByTestId("organization-login-blocked");
    expect(
      within(card).getByRole("heading", { level: 2, name: TWO_FACTOR_HEADING }),
    ).toBeInTheDocument();
    expect(card).toHaveTextContent(
      "An Owner of this organization requires every member to use two-factor authentication (2FA) in the organization and all of its workspaces.",
    );
    expect(
      within(card).getByRole("link", {
        name: "Set up two-factor authentication",
      }),
    ).toHaveAttribute("href", "/settings#security");
    // The org's own settings are not offered, and its navigation stays.
    expect(screen.queryByText("Organization name")).toBeNull();
    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.getByRole("link", { name: "Members" })).toBeInTheDocument();
    // No workspace to leave here.
    expect(
      screen.queryByRole("button", { name: "Leave workspace" }),
    ).toBeNull();
    // An Owner's page also asked for the org's audit-log keys (w1/m657)
    // before the wall went up; the server refuses that read the same way.
    expect(sent.map((op) => op.name).sort()).toEqual([
      "Organization",
      "OrganizationMembers",
      "organizationAPITokens",
    ]);
  });

  it("asks again after enrolling, and gives the page back once the server lets the person in", async () => {
    let enrolled = false;
    const { user, sent } = await renderOrgPage("settings", {
      settings: { requireTwoFactor: true },
      members: () =>
        enrolled
          ? { organizationMembers: [member] }
          : orgRefusal("WORKSPACE_2FA_REQUIRED"),
    });
    await screen.findByTestId("organization-login-blocked");

    enrolled = true;
    await user.click(
      screen.getByRole("button", { name: "I've turned it on, check again" }),
    );

    expect(await screen.findByText("Organization name")).toBeInTheDocument();
    expect(screen.queryByTestId("organization-login-blocked")).toBeNull();
    expect(sent.filter((op) => op.name === "OrganizationMembers")).toHaveLength(
      2,
    );
  });

  it("walls Settings for a Member the org's Google login setting refuses", async () => {
    const { user } = await renderOrgPage("settings", {
      role: "MEMBER",
      settings: { requireGoogleLogin: true },
      members: () => orgRefusal("WORKSPACE_GOOGLE_LOGIN_REQUIRED"),
    });

    const card = await screen.findByTestId("organization-login-blocked");
    expect(
      within(card).getByRole("heading", {
        level: 2,
        name: "Acme requires Google login in all of its workspaces",
      }),
    ).toBeInTheDocument();
    await user.click(
      within(card).getByRole("button", { name: "Sign in with Google" }),
    );
    expect(startGoogleSignIn).toHaveBeenCalledTimes(1);
  });

  // The Members page's own read is refused with the org-only refusal: the
  // block follows from that answer, whatever the cached org said.
  it("walls Members when its own read is refused with only the org named", async () => {
    await renderOrgPage("members", {
      members: () => orgRefusal("WORKSPACE_2FA_REQUIRED"),
    });

    expect(
      await screen.findByRole("heading", {
        level: 2,
        name: TWO_FACTOR_HEADING,
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText("ada@example.com")).toBeNull();
  });

  // A person already known to be refused is not asked again on every org
  // page they open; "check again" is what asks.
  it("does not ask again while the block is recorded", async () => {
    act(() =>
      reportLoginRefusal({
        workspaceId: null,
        organization: { id: "org-1", name: "Acme" },
        requirement: "twoFactor",
      }),
    );
    const { sent } = await renderOrgPage("settings", {
      settings: { requireTwoFactor: true },
      members: () => orgRefusal("WORKSPACE_2FA_REQUIRED"),
    });

    expect(
      await screen.findByTestId("organization-login-blocked"),
    ).toBeInTheDocument();
    expect(sent.map((op) => op.name)).toEqual(["Organization"]);
  });

  it("does not ask while the org requires nothing, and shows the page", async () => {
    const { sent } = await renderOrgPage("settings", {
      members: () => orgRefusal("WORKSPACE_2FA_REQUIRED"),
    });

    expect(await screen.findByText("Organization name")).toBeInTheDocument();
    // The org, and an Owner's audit-log keys (w1/m657); no members check.
    expect(sent.map((op) => op.name).sort()).toEqual([
      "Organization",
      "organizationAPITokens",
    ]);
    expect(screen.queryByTestId("organization-login-blocked")).toBeNull();
  });

  it("lets a person the org's setting does not refuse use the page", async () => {
    const { sent } = await renderOrgPage("settings", {
      settings: { requireTwoFactor: true, requireGoogleLogin: true },
      members: () => ({ organizationMembers: [member] }),
    });

    await waitFor(() =>
      expect(sent.map((op) => op.name)).toContain("OrganizationMembers"),
    );
    expect(screen.getByText("Organization name")).toBeInTheDocument();
    expect(screen.queryByTestId("organization-login-blocked")).toBeNull();
  });
});

// AGENTS.md "Shape parity is mandatory": a navigation to an org page the org
// is already known to refuse previews the blocked card the page will show.
describe("the org pages' pending frame for a refused person (w7/m466)", () => {
  it("draws the same blocked card the page settles to", async () => {
    act(() =>
      reportLoginRefusal({
        workspaceId: null,
        organization: { id: "org-1", name: "Acme" },
        requirement: "twoFactor",
      }),
    );
    const pending = renderAtOrgRoute(
      "settings",
      <OrganizationSettingsRouteSkeleton />,
    );
    const frame = await waitFor(() => {
      const node = pending.container.querySelector(
        '[data-route-skeleton="organization-settings"]',
      );
      expect(node).not.toBeNull();
      return node!;
    });
    const pendingCard = within(frame as HTMLElement).getByTestId(
      "organization-login-blocked",
    );
    expect(pendingCard).toHaveTextContent(TWO_FACTOR_HEADING);
    expect(
      frame.querySelector('[data-testid="organization-name-skeleton"]'),
    ).toBeNull();
    const pendingShape = pendingCard.innerHTML;
    pending.unmount();

    await renderOrgPage("settings", {
      settings: { requireTwoFactor: true },
      members: () => orgRefusal("WORKSPACE_2FA_REQUIRED"),
    });
    const readyCard = await screen.findByTestId("organization-login-blocked");
    expect(readyCard.innerHTML).toBe(pendingShape);
  });
});
