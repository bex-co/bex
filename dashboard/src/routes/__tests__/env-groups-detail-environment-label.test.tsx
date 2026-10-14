import { afterEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, render, screen, within } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import i18n from "@/i18n/init";
import { apolloDefaultOptions } from "@/common/apollo/default-options";

// w4/304: while the workspace scope index is still loading, a valid group's
// Environment row said "Unknown Environment (<id>)" — as if the Environment had
// been deleted — until the lookup landed. The page runs the real
// useEnvGroupScopeIndex against a client whose EnvGroupScopeIndex response the
// test holds, then resolves or fails on demand.

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    workspaces: [{ id: "tea-1", name: "Tea One" }],
    currentWorkspaceId: "tea-1",
    setCurrentWorkspaceId: () => {},
  }),
}));

vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({ services: [], loading: false, error: undefined }),
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

import { EnvGroupDetailPage } from "../env-groups_.$groupId";

function wireGroup(environmentId: string | null) {
  return {
    __typename: "EnvGroup",
    id: "eg1",
    name: "shared",
    ownerId: "tea-1",
    environmentId,
    createdAt: "2026-07-15T12:00:00Z",
    updatedAt: "2026-07-15T13:00:00Z",
    revision: "egr1_test",
    availability: null,
    serviceLinks: [],
    envVars: [],
    secretFiles: [],
  };
}

const project = {
  __typename: "Project",
  id: "prj-1",
  name: "Storefront",
  ownerId: "tea-1",
  createdAt: null,
  serviceIds: [],
  databaseIds: [],
  keyValueIds: [],
};

function environment(id: string) {
  return {
    __typename: "Environment",
    id,
    projectId: "prj-1",
    name: "Production",
    ownerId: "tea-1",
    createdAt: null,
    serviceIds: [],
    databaseIds: [],
    keyValueIds: [],
    envGroupIds: [],
    protectedStatus: "unprotected",
    networkIsolationEnabled: false,
    ipAllowList: [],
    ipAllowListEntries: [],
  };
}

type Held = { next(value: ApolloLink.Result): void; error(e: Error): void };

/** Answers EnvGroup at once; holds every EnvGroupScopeIndex request. */
function scopeHeldClient(environmentId: string | null) {
  const held: Held[] = [];
  const link = new ApolloLink(
    (operation) =>
      new Observable<ApolloLink.Result>((observer) => {
        if (operation.operationName === "EnvGroupScopeIndex") {
          held.push({
            next: (value) => {
              observer.next(value);
              observer.complete();
            },
            error: (e) => observer.error(e),
          });
          return;
        }
        if (operation.operationName === "EnvGroup") {
          observer.next({ data: { envGroup: wireGroup(environmentId) } });
          observer.complete();
          return;
        }
        observer.error(new Error(`unexpected ${operation.operationName}`));
      }),
  );
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link,
    defaultOptions: apolloDefaultOptions,
  });
  const settle = async (fn: (request: Held) => void) => {
    await act(async () => {
      for (const request of held.splice(0)) fn(request);
      await new Promise((resolve) => setTimeout(resolve, 20));
    });
  };
  return {
    client,
    held,
    resolve: (environments: ReturnType<typeof environment>[]) =>
      settle((request) =>
        request.next({
          data: { projects: [project], workspaceEnvironments: environments },
        }),
      ),
    fail: () => settle((request) => request.error(new Error("unavailable"))),
  };
}

function renderDetail(client: ApolloClient) {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/env-groups/$groupId",
    component: EnvGroupDetailPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/env-groups/eg1"] }),
    context: { client: {} as never, session: null },
  });
  return render(
    <ApolloProvider client={client}>
      <RouterProvider router={router} />
    </ApolloProvider>,
  );
}

/** The Environment row's value slot, found through its term. */
async function environmentValue() {
  const term = await screen.findByText("Environment", { selector: "dt" });
  const value = term.nextElementSibling;
  if (!(value instanceof HTMLElement)) throw new Error("no Environment dd");
  return value;
}

/** Every metadata row, in order: [term, value classes]. */
function metadataRows() {
  const list = document.querySelector("dl");
  if (!list) throw new Error("no metadata list");
  return [...list.children].map((row) => [
    row.querySelector("dt")?.textContent,
    row.querySelector("dd")?.className.split(" ").slice(0, 3).join(" "),
  ]);
}

const ROWS = [
  ["Owner", "truncate font-mono text-xs"],
  ["Environment", "truncate font-mono text-xs"],
  ["Created", "truncate font-mono text-xs"],
  ["Updated", "truncate font-mono text-xs"],
];

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("EnvGroupDetailPage Environment label while the scope lookup loads (w4/304)", () => {
  it("announces a neutral loading status, then shows Project / Environment", async () => {
    const apollo = scopeHeldClient("env-1");
    renderDetail(apollo.client);

    const pending = await environmentValue();
    expect(apollo.held.length).toBeGreaterThan(0);
    expect(within(pending).getByRole("status")).toHaveTextContent(
      "Loading environment…",
    );
    expect(pending).toHaveAttribute("title", "Loading environment…");
    expect(screen.queryByText(/Unknown Environment/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Workspace/)).not.toBeInTheDocument();
    expect(pending).not.toHaveTextContent("env-1");
    // Same single-line truncating slot and the same row set as when ready.
    expect(metadataRows()).toEqual(ROWS);

    await apollo.resolve([environment("env-1")]);

    const ready = await environmentValue();
    expect(ready).toHaveTextContent("Storefront / Production");
    expect(ready).toHaveAttribute("title", "Storefront / Production");
    expect(within(ready).queryByRole("status")).not.toBeInTheDocument();
    expect(metadataRows()).toEqual(ROWS);
  });

  it("reports a failed first lookup as a failure, not a missing Environment", async () => {
    const apollo = scopeHeldClient("env-1");
    renderDetail(apollo.client);
    await environmentValue();

    await apollo.fail();

    const value = await environmentValue();
    expect(value).toHaveTextContent("Couldn't load environment");
    expect(within(value).queryByRole("status")).not.toBeInTheDocument();
    expect(screen.queryByText(/Unknown Environment/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Workspace/)).not.toBeInTheDocument();
    expect(metadataRows()).toEqual(ROWS);
  });

  it("keeps Unknown Environment for a successful lookup that lacks the id", async () => {
    const apollo = scopeHeldClient("env-1");
    renderDetail(apollo.client);
    await environmentValue();

    await apollo.resolve([environment("env-other")]);

    const value = await environmentValue();
    expect(value).toHaveTextContent("Unknown Environment (env-1)");
    expect(within(value).queryByRole("status")).not.toBeInTheDocument();
  });

  it("shows Workspace for a null assignment without waiting on the lookup", async () => {
    const apollo = scopeHeldClient(null);
    renderDetail(apollo.client);

    const value = await environmentValue();
    expect(value).toHaveTextContent("Workspace (no Environment)");
    expect(within(value).queryByRole("status")).not.toBeInTheDocument();
  });

  it("localizes the loading status", async () => {
    await i18n.changeLanguage("zh");
    const apollo = scopeHeldClient("env-1");
    renderDetail(apollo.client);

    const term = await screen.findByText("环境", { selector: "dt" });
    const value = term.nextElementSibling as HTMLElement;
    expect(within(value).getByRole("status")).toHaveTextContent(
      "正在加载环境…",
    );
  });
});
