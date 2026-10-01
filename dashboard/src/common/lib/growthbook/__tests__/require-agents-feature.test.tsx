import {
  ApolloClient,
  ApolloLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { act, render, screen, waitFor } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  isRedirect,
  type ParsedLocation,
} from "@tanstack/react-router";
import { describe, expect, it, vi } from "vitest";
import type { Session } from "@ory/client-fetch";
import { AgentsPageSkeleton } from "@/common/components/detail-skeletons";
import type { RouterContext } from "@/common/types/router-context";
import { AGENTS_DASHBOARD_BETA_WORKSPACE_ID as BETA } from "@/config/growthbook";
import type { WorkspacesQuery } from "@/graphql/definitions";
import { Route as AgentsRoute } from "@/routes/agents";
import { Route as AgentDetailRoute } from "@/routes/agents_.$agentSessionId";
import { requireAgentsFeature } from "../require-agents-feature";

function membership(ids: string[]): WorkspacesQuery["workspaces"] {
  return ids.map((id) => ({
    __typename: "Workspace",
    id,
    name: id,
    role: "admin",
    plan: "hobby",
    createdAt: null,
  }));
}

function transport() {
  let reply!: (result: ApolloLink.Result) => void;
  let fail!: (error: Error) => void;
  const request = vi.fn(
    () =>
      new Observable<ApolloLink.Result>((observer) => {
        reply = (result) => {
          observer.next(result);
          observer.complete();
        };
        fail = (error) => observer.error(error);
      }),
  );
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new ApolloLink(request),
  });
  return {
    client,
    request,
    reply: (result: ApolloLink.Result) => reply(result),
    fail: (error: Error) => fail(error),
  };
}

describe("agents workspace eligibility", () => {
  it.each([undefined, null, "tea-deleted", BETA])(
    "resolves membership before accepting cookie %s and reuses the shared cache",
    async (workspaceId) => {
      const server = transport();
      const context = { client: server.client, workspaceId };
      const pending = requireAgentsFeature()({ context });
      expect(server.request).toHaveBeenCalledOnce();
      server.reply({ data: { workspaces: membership([BETA]) } });
      await expect(pending).resolves.toEqual({ workspaceId: BETA });

      await expect(requireAgentsFeature()({ context })).resolves.toEqual({
        workspaceId: BETA,
      });
      expect(server.request).toHaveBeenCalledOnce();
    },
  );

  it.each([
    { cookie: undefined, memberships: ["tea-other", BETA] },
    { cookie: "tea-other", memberships: [BETA, "tea-other"] },
    { cookie: BETA, memberships: ["tea-other"] },
    { cookie: BETA, memberships: [] },
  ])("refuses only a confirmed ineligible selection: %o", async (testCase) => {
    const server = transport();
    const pending = requireAgentsFeature()({
      context: { client: server.client, workspaceId: testCase.cookie },
    });
    server.reply({ data: { workspaces: membership(testCase.memberships) } });
    const error = await Promise.resolve(pending).catch((error) => error);
    expect(isRedirect(error)).toBe(true);
    expect(error.options.to).toBe("/");
  });

  it("preserves an eligible explicit selection even when it is not first", async () => {
    const server = transport();
    const pending = requireAgentsFeature()({
      context: { client: server.client, workspaceId: BETA },
    });
    server.reply({ data: { workspaces: membership(["tea-other", BETA]) } });
    await expect(pending).resolves.toEqual({ workspaceId: BETA });
  });

  it.each(["transport", "graphql", "missing membership"])(
    "keeps %s failure distinct from confirmed ineligibility",
    async (failure) => {
      const server = transport();
      const pending = requireAgentsFeature()({
        context: { client: server.client, workspaceId: BETA },
      });
      if (failure === "transport") server.fail(new Error("offline"));
      else if (failure === "graphql")
        server.reply({
          data: { workspaces: [] },
          errors: [{ message: "memberships unavailable" }],
        });
      else server.reply({ data: { workspaces: null } });

      const error = await Promise.resolve(pending).catch((error) => error);
      expect(error).toBeInstanceOf(Error);
      expect(isRedirect(error)).toBe(false);

      const recovered = requireAgentsFeature()({
        context: { client: server.client, workspaceId: BETA },
      });
      expect(server.request).toHaveBeenCalledTimes(2);
      server.reply({ data: { workspaces: membership([BETA]) } });
      await expect(recovered).resolves.toEqual({ workspaceId: BETA });
    },
  );
});

describe.each([
  { path: "/agents", route: AgentsRoute },
  { path: "/agents/$agentSessionId", route: AgentDetailRoute },
])("cold $path route", ({ path, route }) => {
  it("holds its destination and loader until the actual beforeLoad resolves membership", async () => {
    const server = transport();
    const initialHref =
      path.replace("$agentSessionId", "ags-test") + "?archived=all#history";
    const loader = vi.fn(
      ({ context }: { context: RouterContext }) => context.workspaceId,
    );
    const root = createRootRouteWithContext<RouterContext>()({
      component: Outlet,
    });
    const home = createRoute({
      getParentRoute: () => root,
      path: "/",
      component: () => <div>Home</div>,
    });
    const agents = createRoute({
      getParentRoute: () => root,
      path,
      beforeLoad: ({ context, location }) => {
        // Exercise the production guard while adapting its file-route parent
        // types to this isolated router's root. It reads only these two fields.
        const beforeLoad = route.options.beforeLoad as (options: {
          context: RouterContext;
          location: ParsedLocation;
        }) => Promise<Pick<RouterContext, "workspaceId">>;
        return beforeLoad({ context, location });
      },
      loader,
      pendingComponent: () => <AgentsPageSkeleton mode="list" />,
      component: () => <div>Eligible destination</div>,
    });
    const router = createRouter({
      routeTree: root.addChildren([home, agents]),
      history: createMemoryHistory({ initialEntries: [initialHref] }),
      context: {
        client: server.client,
        workspaceId: "tea-deleted",
        session: { id: "session-test" } as Session,
      },
      defaultPendingMs: 0,
      defaultPendingMinMs: 0,
    });
    render(<RouterProvider router={router} />);

    await waitFor(() => expect(server.request).toHaveBeenCalledOnce());
    expect(loader).not.toHaveBeenCalled();
    expect(router.state.location.href).toBe(initialHref);
    expect(
      await screen.findByTestId("agents-page-skeleton"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Home")).not.toBeInTheDocument();

    await act(async () => {
      server.reply({ data: { workspaces: membership([BETA]) } });
    });

    expect(await screen.findByText("Eligible destination")).toBeInTheDocument();
    expect(loader).toHaveBeenCalledOnce();
    expect(loader.mock.calls[0][0].context.workspaceId).toBe(BETA);
    expect(router.state.location.href).toBe(initialHref);
  });
});
