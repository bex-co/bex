import type { ReactElement } from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { LoginRequirementNotice } from "@/features/workspaces/components/login-requirement-compliance";
import { WorkspaceLoginRequirementCard } from "@/features/workspaces/components/workspace-login-requirement-card";
import { WorkspaceLoginRequirementCardSkeleton } from "@/features/workspaces/components/workspace-login-requirement-card-skeleton";
import type { WorkspaceView } from "@/features/workspaces/types";
import {
  WorkspaceLoginRequirementImpactDocument,
  WorkspaceNonCompliantMembersDocument,
} from "@/graphql/definitions";

// Once 2FA or Google login is enforced (w1/m647,
// docs/render-artifacts/workspace-2fa-reminders.md): Render's notice under the
// switch, the members dialog, the 2FA reminders with their daily limit, and
// the confirmation's reminder line, in Render's words.

const mutate = vi.hoisted(() => vi.fn());
const query = vi.hoisted(() =>
  vi.fn<
    (
      document: unknown,
      options: { skip?: boolean },
    ) => { data?: unknown; error?: Error }
  >(),
);
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate, { loading: false }],
  useQuery: query,
}));
const toastSuccess = vi.hoisted(() => vi.fn());
vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: vi.fn() },
}));

const WORKSPACE: WorkspaceView = {
  id: "tea-acme",
  name: "Acme",
  slug: "acme",
  plan: "pro",
  role: "admin",
  createdAt: null,
  requireTwoFactor: true,
  requireGoogleLogin: true,
  organization: null,
  primary: false,
  privacy: null,
};

const member = (n: number) => ({
  __typename: "NonCompliantMember" as const,
  userId: `own-${n}`,
  email: `member${n}@acme.test`,
  name: "",
});

/** What the enforced notice's read answers; undefined: still reading. */
let nonCompliant: { data?: unknown; error?: Error } = {};
/** What the confirmation's read answers. */
let impact: { data?: unknown; error?: Error } = {};

function answer(document: unknown, options: { skip?: boolean }) {
  if (options.skip) return {};
  if (document === WorkspaceNonCompliantMembersDocument) return nonCompliant;
  if (document === WorkspaceLoginRequirementImpactDocument) return impact;
  return {};
}

function listing(count: number) {
  return {
    data: {
      workspaceNonCompliantMembers: Array.from({ length: count }, (_, i) =>
        member(i + 1),
      ),
    },
  };
}

async function renderCard(ui: ReactElement) {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => ui,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

function card(
  requirement: "twoFactor" | "googleLogin",
  workspace: WorkspaceView = WORKSPACE,
) {
  return (
    <WorkspaceLoginRequirementCard
      requirement={requirement}
      workspace={workspace}
      onChangePlan={vi.fn()}
    />
  );
}

function notice() {
  return document.querySelector("[data-login-requirement-notice]");
}

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(
    mockCapabilities({ role: "ADMIN", canManage: true }),
  );
  mutate.mockReset();
  mutate.mockResolvedValue({ data: { sendWorkspaceTwoFactorReminders: 2 } });
  toastSuccess.mockReset();
  query.mockReset();
  query.mockImplementation(answer);
  nonCompliant = {};
  impact = {};
});

describe("the enforced notice (w1/m647)", () => {
  it("says how many members have not enabled 2FA, with View members", async () => {
    nonCompliant = listing(2);
    await renderCard(card("twoFactor"));

    expect(notice()).toHaveTextContent(
      "2 members of your team have not enabled two-factor authentication yet.",
    );
    expect(notice()!.querySelector("strong")).toHaveTextContent("2 members");
    expect(
      within(notice() as HTMLElement).getByRole("button", {
        name: "View members",
      }),
    ).toBeInTheDocument();
    expect(query).toHaveBeenCalledWith(
      WorkspaceNonCompliantMembersDocument,
      expect.objectContaining({
        variables: { workspaceId: "tea-acme", requirement: "TWO_FACTOR" },
        skip: false,
      }),
    );
  });

  it("speaks of one member in the singular", async () => {
    nonCompliant = listing(1);
    await renderCard(card("twoFactor"));
    expect(notice()).toHaveTextContent(
      "1 member of your team has not enabled two-factor authentication yet.",
    );
    expect(screen.getByRole("button", { name: "View member" })).toBeVisible();
  });

  it("says so when every member has enabled it", async () => {
    nonCompliant = listing(0);
    await renderCard(card("twoFactor"));
    expect(notice()).toHaveTextContent(
      "All members of your team have enabled two-factor authentication!",
    );
    expect(notice()!.querySelector("strong")).toHaveTextContent("All members");
    expect(within(notice() as HTMLElement).queryByRole("button")).toBeNull();
  });

  it("shows nothing when the read fails, never All members", async () => {
    nonCompliant = { error: new Error("could not check") };
    await renderCard(card("twoFactor"));
    expect(notice()).toBeNull();
  });

  it("holds the notice's box while the read is in flight", async () => {
    await renderCard(card("twoFactor"));
    expect(notice()).toHaveAttribute(
      "data-login-requirement-notice",
      "pending",
    );
    expect(notice()).toHaveAttribute("aria-hidden", "true");
  });

  it("is not drawn where the requirement is off, or for a non-admin", async () => {
    nonCompliant = listing(2);
    const { unmount } = await renderCard(
      card("twoFactor", { ...WORKSPACE, requireTwoFactor: false }),
    );
    expect(notice()).toBeNull();
    unmount();

    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    await renderCard(card("twoFactor"));
    expect(notice()).toBeNull();
    expect(query).not.toHaveBeenCalledWith(
      WorkspaceNonCompliantMembersDocument,
      expect.objectContaining({ skip: false }),
    );
  });

  it("is drawn under an org's requirement too", async () => {
    nonCompliant = listing(1);
    await renderCard(
      card("twoFactor", {
        ...WORKSPACE,
        requireTwoFactor: false,
        organization: {
          id: "org-1",
          name: "Acme Org",
          requireTwoFactor: true,
          requireGoogleLogin: false,
        } as WorkspaceView["organization"],
      }),
    );
    expect(notice()).toHaveTextContent("1 member of your team has not");
  });

  it("reads Google login's in its own words", async () => {
    nonCompliant = listing(3);
    await renderCard(card("googleLogin"));
    expect(notice()).toHaveTextContent(
      "3 members of your team have not signed in with Google yet.",
    );
    expect(query).toHaveBeenCalledWith(
      WorkspaceNonCompliantMembersDocument,
      expect.objectContaining({
        variables: { workspaceId: "tea-acme", requirement: "GOOGLE_LOGIN" },
      }),
    );
  });

  it("says all of them signed in with Google", async () => {
    nonCompliant = listing(0);
    await renderCard(card("googleLogin"));
    expect(notice()).toHaveTextContent(
      "All members of your team have signed in with Google!",
    );
  });
});

describe("the members dialog (w1/m647)", () => {
  async function openList(
    requirement: "twoFactor" | "googleLogin",
    count: number,
  ) {
    const user = userEvent.setup();
    nonCompliant = listing(count);
    await renderCard(card(requirement));
    await user.click(
      screen.getByRole("button", {
        name: count > 1 ? "View members" : "View member",
      }),
    );
    return { user, dialog: await screen.findByRole("dialog") };
  }

  it("lists the members without 2FA and offers the reminders", async () => {
    const { dialog } = await openList("twoFactor", 2);
    expect(
      within(dialog).getByRole("heading", {
        name: "Members without two-factor authentication",
      }),
    ).toBeVisible();
    expect(dialog).toHaveTextContent(
      "These members are currently prohibited from accessing this team workspace and the resources within it.",
    );
    const list = within(dialog).getByRole("list", {
      name: "Members without two-factor authentication",
    });
    expect(
      within(list)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual(["member1@acme.test", "member2@acme.test"]);
    expect(dialog).toHaveTextContent("Remind these users to enable 2FA?");
    expect(dialog).toHaveTextContent(
      "Send out email reminders to these users.",
    );
    expect(
      within(dialog).getByRole("button", { name: "Send reminders" }),
    ).toBeVisible();
  });

  it("speaks of one member in the singular", async () => {
    const { dialog } = await openList("twoFactor", 1);
    expect(dialog).toHaveTextContent(
      "This member is currently prohibited from accessing this team workspace and the resources within it.",
    );
    expect(dialog).toHaveTextContent("Remind this user to enable 2FA?");
    expect(dialog).toHaveTextContent(
      "Send out an email reminder to this user.",
    );
    expect(
      within(dialog).getByRole("button", { name: "Send reminder" }),
    ).toBeVisible();
  });

  it("sends the reminders, closes and says so", async () => {
    const { user, dialog } = await openList("twoFactor", 2);
    await user.click(
      within(dialog).getByRole("button", { name: "Send reminders" }),
    );
    expect(mutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-acme" },
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(toastSuccess).toHaveBeenCalledWith(
      "Email reminders sent to members without two-factor authentication!",
    );
  });

  it("shows Render's limit once today's reminders went out", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("WORKSPACE_2FA_REMINDER_LIMIT", {
        nextReminderAt: "2026-11-27T00:00:00Z",
      }),
    );
    const { user, dialog } = await openList("twoFactor", 2);
    await user.click(
      within(dialog).getByRole("button", { name: "Send reminders" }),
    );
    const limit = await screen.findByRole("dialog", {
      name: "Remind limit has been reached!",
    });
    expect(limit).toHaveTextContent(
      "It looks like we've already sent out reminders for today. You can send reminders again tomorrow.",
    );
    expect(
      screen.queryByRole("dialog", {
        name: "Members without two-factor authentication",
      }),
    ).toBeNull();
    expect(toastSuccess).not.toHaveBeenCalled();
    await user.click(
      within(limit).getAllByRole("button", { name: "Close" })[0],
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("keeps the dialog open with Render's apology on any other failure", async () => {
    mutate.mockRejectedValue(new Error("network down"));
    const { user, dialog } = await openList("twoFactor", 2);
    await user.click(
      within(dialog).getByRole("button", { name: "Send reminders" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "We're sorry. We weren't able to send reminders to these team members. Try again?",
    );
    expect(screen.getByRole("dialog")).toBe(dialog);
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("lists Google login's members with no reminder, as on Render", async () => {
    const { dialog } = await openList("googleLogin", 2);
    expect(
      within(dialog).getByRole("heading", {
        name: "Members without Google sign-on",
      }),
    ).toBeVisible();
    expect(dialog).not.toHaveTextContent("Remind");
    expect(within(dialog).queryByRole("button", { name: /Send/ })).toBeNull();
  });
});

describe("the 2FA confirmation's reminder line (w1/m647)", () => {
  const losing = (n: number) => ({
    data: {
      workspaceLoginRequirementImpact: {
        members: Array.from({ length: n }, (_, i) => ({
          userId: `own-${i}`,
          email: `m${i}@acme.test`,
          name: "",
          losesAccess: true,
          apiKeys: [],
          sshKeys: [],
        })),
        unattributedApiKeys: [],
      },
    },
  });
  const off = {
    ...WORKSPACE,
    requireTwoFactor: false,
    requireGoogleLogin: false,
  };

  it("says the members losing access will be reminded", async () => {
    impact = losing(2);
    const user = userEvent.setup();
    await renderCard(card("twoFactor", off));
    await user.click(screen.getByRole("switch"));
    expect(await screen.findByRole("alertdialog")).toHaveTextContent(
      "Reminders to enable 2FA will be sent to these members.",
    );
  });

  it("in the singular for one", async () => {
    impact = losing(1);
    const user = userEvent.setup();
    await renderCard(card("twoFactor", off));
    await user.click(screen.getByRole("switch"));
    expect(await screen.findByRole("alertdialog")).toHaveTextContent(
      "A reminder to enable 2FA will be sent to this member.",
    );
  });

  it("is absent with nobody losing access, and for Google login", async () => {
    impact = losing(0);
    const user = userEvent.setup();
    const { unmount } = await renderCard(card("twoFactor", off));
    await user.click(screen.getByRole("switch"));
    expect(await screen.findByRole("alertdialog")).not.toHaveTextContent(
      "will be sent",
    );
    unmount();

    impact = losing(2);
    await renderCard(card("googleLogin", off));
    await user.click(screen.getByRole("switch"));
    expect(await screen.findByRole("alertdialog")).not.toHaveTextContent(
      "will be sent",
    );
  });
});

// AGENTS.md: a preview holds the ready state's geometry. The pending notice is
// the ready one's box, the same words laid out without ink, and the same
// button; the card skeleton draws it under an enforced requirement.
describe("the notice's pending shape (w1/m647)", () => {
  it.each([0, 1, 4])("matches the ready notice for %i members", (count) => {
    const { container: ready } = render(
      <LoginRequirementNotice requirement="twoFactor" count={count} />,
    );
    const { container: pending } = render(
      <LoginRequirementNotice requirement="twoFactor" count={count} pending />,
    );
    const readyBox = ready.firstElementChild!;
    const pendingBox = pending.firstElementChild!;
    expect(pendingBox.className).toBe(readyBox.className);
    expect(pendingBox.lastElementChild!.className).toBe(
      readyBox.lastElementChild!.className,
    );
    // The same words, invisible, so they wrap the same at every width.
    expect(pendingBox.querySelector("p .invisible")?.textContent).toBe(
      readyBox.querySelector("p")?.textContent,
    );
    expect(pendingBox.querySelectorAll("button, [class*='h-8']").length).toBe(
      readyBox.querySelectorAll("button, [class*='h-8']").length,
    );
  });

  it("is in the card skeleton where the card will draw it", () => {
    const { container, rerender } = render(
      <WorkspaceLoginRequirementCardSkeleton
        requirement="twoFactor"
        notice={2}
      />,
    );
    expect(
      container.querySelector('[data-skeleton-region="two-factor-notice"]'),
    ).toHaveTextContent(
      "2 members of your team have not enabled two-factor authentication yet.",
    );
    rerender(<WorkspaceLoginRequirementCardSkeleton requirement="twoFactor" />);
    expect(
      container.querySelector('[data-skeleton-region="two-factor-notice"]'),
    ).toBeNull();
  });
});
