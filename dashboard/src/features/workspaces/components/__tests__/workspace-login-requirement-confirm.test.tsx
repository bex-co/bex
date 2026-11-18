import type { ReactElement } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { shape } from "@/test/card-shape";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { LoginRequirementImpactList } from "@/features/workspaces/components/login-requirement-impact-list";
import { WorkspaceLoginRequirementCard } from "@/features/workspaces/components/workspace-login-requirement-card";
import type { LoginRequirementImpactState } from "@/features/workspaces/hooks/use-login-requirement-impact";
import type { WorkspaceView } from "@/features/workspaces/types";

// The confirmation before an admin turns a workspace login requirement on
// (w7/m471, docs/ADR012-auth.md 9c): who it would block, from bex-api's
// workspaceLoginRequirementImpact, as Render's SSO enforcement lists them.

const mutate = vi.hoisted(() => vi.fn());
type QueryResult = { data?: unknown; error?: Error };
const impactQuery = vi.hoisted(() =>
  vi.fn<(document: unknown, options: Record<string, unknown>) => QueryResult>(),
);
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate, { loading: false }],
  useQuery: impactQuery,
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const WORKSPACE: WorkspaceView = {
  id: "tea-acme",
  name: "Acme",
  slug: "acme",
  plan: "pro",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: null,
  primary: false,
  privacy: null,
};

/** One unenrolled member with a key of each kind, and a key nobody made. */
const LISTED = {
  workspaceLoginRequirementImpact: {
    __typename: "LoginRequirementImpact",
    members: [
      {
        __typename: "LoginRequirementImpactMember",
        userId: "own-1",
        email: "two@acme.test",
        name: "Two",
        losesAccess: true,
        apiKeys: [
          {
            __typename: "LoginRequirementImpactApiKey",
            id: "key-1",
            name: "deploy bot",
          },
        ],
        sshKeys: [
          {
            __typename: "LoginRequirementImpactSshKey",
            id: "ssk-1",
            name: "laptop",
            fingerprint: "SHA256:abc",
          },
        ],
      },
      {
        __typename: "LoginRequirementImpactMember",
        userId: "own-2",
        email: "dev@acme.test",
        name: "",
        losesAccess: false,
        apiKeys: [
          {
            __typename: "LoginRequirementImpactApiKey",
            id: "key-2",
            name: "ci",
          },
        ],
        sshKeys: [],
      },
    ],
    unattributedApiKeys: [
      {
        __typename: "LoginRequirementImpactApiKey",
        id: "key-3",
        name: "legacy",
      },
    ],
  },
};

const NOBODY = {
  workspaceLoginRequirementImpact: {
    __typename: "LoginRequirementImpact",
    members: [],
    unattributedApiKeys: [],
  },
};

let answer: QueryResult = {};

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

function card(requirement: "twoFactor" | "googleLogin", workspace = WORKSPACE) {
  return (
    <WorkspaceLoginRequirementCard
      requirement={requirement}
      workspace={workspace}
      onChangePlan={vi.fn()}
    />
  );
}

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(
    mockCapabilities({ role: "ADMIN", canManage: true }),
  );
  mutate.mockReset();
  mutate.mockResolvedValue({ data: {} });
  answer = {};
  impactQuery.mockReset();
  impactQuery.mockImplementation((_document, options) =>
    options.skip ? {} : answer,
  );
});

describe("WorkspaceLoginRequirementCard — the enable confirmation (w7/m471)", () => {
  it("lists each member it would block, with their keys, before anything is written", async () => {
    answer = { data: LISTED };
    const user = userEvent.setup();
    await renderCard(card("twoFactor"));

    // Closed, it reads nothing.
    expect(impactQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({ skip: true }),
    );
    await user.click(screen.getByRole("switch"));

    const dialog = await screen.findByRole("alertdialog", {
      name: "Require two-factor authentication in Acme?",
    });
    expect(impactQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: { workspaceId: "tea-acme", requirement: "TWO_FACTOR" },
        skip: false,
        fetchPolicy: "network-only",
      }),
    );
    expect(mutate).not.toHaveBeenCalled();
    const list = within(dialog).getByRole("region", {
      name: "Who would lose access",
    });
    expect(list).toHaveTextContent(
      "Each listed member will lose access until they turn on two-factor authentication.",
    );
    const [two, dev] = within(list).getAllByRole("listitem").slice(0, 2);
    expect(two).toHaveTextContent("Two");
    expect(two).toHaveTextContent("Loses access");
    expect(two).toHaveTextContent("two@acme.test");
    expect(two).toHaveTextContent("API key deploy bot");
    expect(two).toHaveTextContent("SSH key laptop SHA256:abc");
    // A member with no name is shown by their email, keeping access.
    expect(dev).toHaveTextContent("dev@acme.test");
    expect(dev).toHaveTextContent("Keeps access");
    expect(dev).toHaveTextContent("API key ci");
    expect(list).toHaveTextContent(
      "API keys with no recorded creator, or whose creator is no longer a member, will stop working:",
    );
    expect(list).toHaveTextContent("API key legacy");
    // 2FA has no sign-in to explain.
    expect(dialog).not.toHaveTextContent(/asked to sign in with Google/);

    await user.click(within(dialog).getByRole("button", { name: "Require" }));
    expect(mutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-acme", enabled: true },
    });
  });

  it("says nobody would be blocked when every member meets it", async () => {
    answer = { data: NOBODY };
    const user = userEvent.setup();
    await renderCard(card("twoFactor"));
    await user.click(screen.getByRole("switch"));

    const list = await screen.findByRole("region", {
      name: "Who would lose access",
    });
    expect(list).toHaveTextContent(
      "Nobody would lose access: every member meets this requirement, and no API key or SSH key would stop working.",
    );
    expect(within(list).queryAllByRole("listitem")).toHaveLength(0);
  });

  it("keeps confirm open while it checks", async () => {
    const user = userEvent.setup();
    await renderCard(card("twoFactor"));
    await user.click(screen.getByRole("switch"));

    const list = await screen.findByRole("region", {
      name: "Who would lose access",
    });
    expect(list).toHaveAttribute("aria-busy", "true");
    expect(list).toHaveTextContent("Checking who would lose access…");
    expect(screen.getByRole("button", { name: "Require" })).toBeEnabled();
  });

  // A check that cannot answer is never shown as nobody.
  it("says it could not check, and still lets the admin confirm", async () => {
    answer = { error: new Error("authorization service unavailable") };
    const user = userEvent.setup();
    await renderCard(card("twoFactor"));
    await user.click(screen.getByRole("switch"));

    const list = await screen.findByRole("region", {
      name: "Who would lose access",
    });
    expect(within(list).getByRole("status")).toHaveTextContent(
      "Couldn't check who would lose access. You can still turn it on.",
    );
    expect(list).not.toHaveTextContent(/Nobody would lose access/);
    await user.click(screen.getByRole("button", { name: "Require" }));
    expect(mutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-acme", enabled: true },
    });
  });

  it("writes nothing and leaves the switch off when cancelled", async () => {
    answer = { data: LISTED };
    const user = userEvent.setup();
    await renderCard(card("twoFactor"));
    await user.click(screen.getByRole("switch"));
    await user.click(await screen.findByRole("button", { name: "Cancel" }));

    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(mutate).not.toHaveBeenCalled();
    expect(screen.getByRole("switch")).not.toBeChecked();
  });

  it("asks for Google login with its own words and the sign-in line", async () => {
    answer = {
      data: {
        workspaceLoginRequirementImpact: {
          ...NOBODY.workspaceLoginRequirementImpact,
        },
      },
    };
    const user = userEvent.setup();
    await renderCard(card("googleLogin"));
    await user.click(screen.getByRole("switch"));

    const dialog = await screen.findByRole("alertdialog", {
      name: "Require Google login in Acme?",
    });
    expect(impactQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: { workspaceId: "tea-acme", requirement: "GOOGLE_LOGIN" },
      }),
    );
    expect(dialog).toHaveTextContent(
      "Members who meet it but are signed in another way right now will be asked to sign in with Google.",
    );
  });

  it.each(["twoFactor", "googleLogin"] as const)(
    "turns %s off at once, without asking",
    async (requirement) => {
      const user = userEvent.setup();
      await renderCard(
        card(requirement, {
          ...WORKSPACE,
          requireTwoFactor: true,
          requireGoogleLogin: true,
        }),
      );
      await user.click(screen.getByRole("switch"));

      expect(screen.queryByRole("alertdialog")).toBeNull();
      expect(mutate).toHaveBeenCalledWith({
        variables: { workspaceId: "tea-acme", enabled: false },
      });
      expect(impactQuery).not.toHaveBeenCalledWith(
        expect.anything(),
        expect.objectContaining({ skip: false }),
      );
    },
  );

  it("never asks on a card below Pro or locked by the org", async () => {
    const user = userEvent.setup();
    await renderCard(card("twoFactor", { ...WORKSPACE, plan: "hobby" }));
    await user.click(screen.getByRole("switch"));
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });
});

// AGENTS.md "Shape parity is mandatory": the dialog keeps its geometry while
// the answer arrives. Every state draws inside the same fixed-height region,
// and nothing outside it changes.
describe("LoginRequirementImpactList — geometry (w7/m471)", () => {
  const STATES: Record<string, LoginRequirementImpactState> = {
    loading: { status: "loading" },
    error: { status: "error" },
    nobody: { status: "ready", members: [], unattributedApiKeys: [] },
    listed: {
      status: "ready",
      members: LISTED.workspaceLoginRequirementImpact.members,
      unattributedApiKeys:
        LISTED.workspaceLoginRequirementImpact.unattributedApiKeys,
    },
  };

  it.each(["twoFactor", "googleLogin"] as const)(
    "draws %s's loading, failed, empty and listed states in one fixed box",
    (requirement) => {
      const frames = Object.entries(STATES).map(([name, state]) => {
        const { container, unmount } = render(
          <LoginRequirementImpactList
            requirement={requirement}
            state={state}
          />,
        );
        const root = container.firstElementChild!;
        const region = root.querySelector("[data-login-requirement-impact]")!;
        // Everything but the region's own contents: the box and the lines
        // around it.
        const frame = {
          name,
          root: root.className,
          region: region.className,
          siblings: [...root.children].map((child) =>
            child === region ? "region" : shape(child),
          ),
        };
        unmount();
        return frame;
      });
      for (const frame of frames) {
        // The dialog is a grid: without min-w-0 an unbroken fingerprint
        // widened it on dev-7 (w7/m471 t006) instead of truncating.
        expect(frame.root).toContain("min-w-0");
        expect(frame.region).toContain("h-48");
        expect(frame.region).toContain("overflow-y-auto");
        expect({ ...frame, name: "" }).toEqual({ ...frames[0], name: "" });
      }
    },
  );

  it("lists the skeleton's rows where the listed state draws its members", () => {
    const { container } = render(
      <LoginRequirementImpactList
        requirement="twoFactor"
        state={{ status: "loading" }}
      />,
    );
    const rows = container.querySelectorAll("li");
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(row.className).toContain("px-3 py-2");
    }
  });
});
