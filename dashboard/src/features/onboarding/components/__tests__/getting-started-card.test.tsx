import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { GettingStartedCard } from "../getting-started-card";

const state = vi.hoisted(() => ({
  workspaceId: "tea-a" as string | null,
  gitConnected: false,
  members: 1,
  invites: 0,
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: state.workspaceId }),
}));
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnections: () => ({ connected: state.gitConnected }),
}));
vi.mock("@/features/team/hooks/use-team", () => ({
  useTeam: () => ({
    members: Array.from({ length: state.members }, (_, i) => ({ id: `m${i}` })),
    invites: Array.from({ length: state.invites }, (_, i) => ({ id: `i${i}` })),
  }),
}));

function renderCard() {
  const rootRoute = createRootRoute();
  const home = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <GettingStartedCard />,
  });
  const routes = ["/services/new", "/settings", "/workspace/settings"].map(
    (path) =>
      createRoute({
        getParentRoute: () => rootRoute,
        path,
        component: () => <p>landed on {path}</p>,
      }),
  );
  const router = createRouter({
    routeTree: rootRoute.addChildren([home, ...routes]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

// src/test/setup.ts installs a no-op localStorage; persistence is the point
// of these tests, so give each one a real in-memory Storage.
function memoryStorage(): Storage {
  const items = new Map<string, string>();
  return {
    get length() {
      return items.size;
    },
    clear: () => items.clear(),
    getItem: (k) => items.get(k) ?? null,
    key: (i) => [...items.keys()][i] ?? null,
    removeItem: (k) => void items.delete(k),
    setItem: (k, v) => void items.set(k, String(v)),
  };
}

beforeEach(() => {
  vi.stubGlobal("localStorage", memoryStorage());
  state.workspaceId = "tea-a";
  state.gitConnected = false;
  state.members = 1;
  state.invites = 0;
});

describe("GettingStartedCard (ADR075 D2)", () => {
  it("links every pending step to the flow that completes it", async () => {
    const { router } = renderCard();
    const card = await screen.findByTestId("getting-started-card");
    expect(within(card).getAllByLabelText("Not done yet")).toHaveLength(3);

    await userEvent.click(
      within(card).getByRole("link", { name: "Connect GitHub to deploy from a repository" }),
    );
    expect(router.state.location.pathname).toBe("/settings");
    expect(router.state.location.hash).toBe("integrations");
  });

  it("marks GitHub and teammate steps done from the workspace's real state", async () => {
    state.gitConnected = true;
    state.invites = 1; // a pending invite counts: the step is "invite", not "accepted"
    renderCard();
    const card = await screen.findByTestId("getting-started-card");
    expect(within(card).getAllByLabelText("Done")).toHaveLength(2);
    expect(
      within(card).queryByRole("link", { name: "Invite a teammate" }),
    ).not.toBeInTheDocument();
    expect(
      within(card).getByRole("link", { name: "Create your first service" }),
    ).toHaveAttribute("href", "/services/new");
  });

  it("stays dismissed across a reload, per workspace", async () => {
    const first = renderCard();
    await userEvent.click(
      await screen.findByRole("button", { name: "Dismiss getting started" }),
    );
    expect(screen.queryByTestId("getting-started-card")).not.toBeInTheDocument();
    first.unmount();

    renderCard(); // reload: same workspace stays dismissed
    await act(async () => {});
    expect(screen.queryByTestId("getting-started-card")).not.toBeInTheDocument();
  });

  it("shows another workspace's card even after one was dismissed", async () => {
    window.localStorage.setItem("bex.gettingStarted.dismissed.tea-a", "1");
    state.workspaceId = "tea-b";
    renderCard();
    expect(await screen.findByTestId("getting-started-card")).toBeInTheDocument();
  });

  it("renders nothing before a workspace is known", async () => {
    state.workspaceId = null;
    renderCard();
    await act(async () => {});
    expect(screen.queryByTestId("getting-started-card")).not.toBeInTheDocument();
  });
});
