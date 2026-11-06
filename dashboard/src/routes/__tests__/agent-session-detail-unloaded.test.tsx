import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { CombinedGraphQLErrors } from "@apollo/client";

// w4/523: an unknown session link drew an empty content area for ~5 s (no
// session, no query in flight, no error: the cache keeps a refused read's null
// session but not its error), and the tab read "Page not found" while the body
// then said it couldn't load.

const hook = vi.hoisted(() => ({
  state: { session: null, loading: false, error: undefined } as {
    session: null;
    loading: boolean;
    error: unknown;
  },
}));

vi.mock("@/features/agent-sessions/hooks/use-agent-session", () => ({
  useAgentSession: () => ({ ...hook.state, refetch: async () => undefined }),
}));

vi.mock("@/common/hooks/use-loader-error-retry", () => ({
  useLoaderErrorRetry: () => undefined,
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

import { Route } from "../agents_.$agentSessionId";

async function renderPage() {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/agents/$agentSessionId",
    component: Route.options.component,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/agents/ags-x"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  await router.load();
}

describe("agent session detail with no session loaded", () => {
  it("shows the load-error state at once, not an empty page", async () => {
    hook.state = { session: null, loading: false, error: undefined };
    await renderPage();
    expect(
      await screen.findByText("Couldn't load this session"),
    ).toBeInTheDocument();
  });

  it("titles a refused read as an error, and only NOT_FOUND as not found", async () => {
    type Loader = (args: unknown) => Promise<{ state: string }>;
    const loader = Route.options.loader as unknown as Loader;
    const load = (answer: unknown) =>
      loader({
        context: { client: { query: async () => answer } },
        params: { agentSessionId: "ags-x" },
        cause: "enter",
      });
    const refused = (code: string) =>
      new CombinedGraphQLErrors({
        errors: [{ message: code.toLowerCase(), extensions: { code } }],
      });
    // A cache-served re-run: the null session without its error.
    expect(await load({ data: { agentSession: null } })).toEqual({
      state: "error",
    });
    expect(
      await load({ data: { agentSession: null }, error: refused("FORBIDDEN") }),
    ).toEqual({ state: "error" });
    expect(
      await load({
        data: { agentSession: null },
        error: refused("AGENT_SESSION_NOT_FOUND"),
      }),
    ).toEqual({ state: "not-found" });
  });
});
