import { beforeAll, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { Observable } from "rxjs";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { EnvGroupDocument } from "@/graphql/definitions";

// w1/m153 t003: a background poll of the EnvGroup query over cached data must
// not reach the detail page's Environment editor as `loading`. Before the fix it
// did, so the editor swapped its rows for a skeleton every 30 s and the draft
// row being typed in unmounted and lost focus.
//
// The page runs the real useEnvGroup (and the real editor under it) against a
// real client built with the dashboard's apolloDefaultOptions; only the
// unrelated side panels' data hooks and the chrome are stubbed.

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

const scopeState = {
  projects: [],
  environments: [],
  byId: new Map<string, { id: string; name: string }>(),
  serviceEnvironmentById: new Map<string, string>(),
  loading: false,
  error: undefined,
};

vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => ({
  useEnvGroupScopeIndex: () => scopeState,
  useWorkspaceEnvironmentIndex: () => scopeState,
}));

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

const wireGroup = {
  __typename: "EnvGroup",
  id: "eg1",
  name: "shared",
  ownerId: "tea-1",
  environmentId: null,
  createdAt: "2026-07-15T12:00:00Z",
  updatedAt: "2026-07-15T13:00:00Z",
  revision: "egr1_test",
  availability: null,
  serviceLinks: [],
  envVars: [{ __typename: "EnvGroupVar", key: "FOO" }],
  secretFiles: [],
};

/**
 * A MockLink behind a gate: while held, requests are recorded but not answered,
 * so a refetch stays in flight for as long as the test needs to inspect it.
 */
function gatedClient() {
  const requests: string[] = [];
  let gate: Promise<void> | null = null;
  let open = () => {};
  const hold = new ApolloLink(
    (operation, forward) =>
      new Observable<ApolloLink.Result>((observer) => {
        requests.push(operation.operationName ?? "");
        let closed = false;
        let subscription: { unsubscribe(): void } | undefined;
        void (gate ?? Promise.resolve()).then(() => {
          if (!closed) subscription = forward(operation).subscribe(observer);
        });
        return () => {
          closed = true;
          subscription?.unsubscribe();
        };
      }),
  );
  const mocks = new MockLink(
    [
      {
        request: { query: EnvGroupDocument, variables: { id: "eg1" } },
        result: { data: { envGroup: wireGroup } },
        maxUsageCount: Number.POSITIVE_INFINITY,
      },
    ],
    { defaultOptions: { delay: 0 } },
  );
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: ApolloLink.from([hold, mocks]),
    defaultOptions: apolloDefaultOptions,
  });
  return {
    client,
    requests,
    holdResponses() {
      gate = new Promise((resolve) => {
        open = resolve;
      });
    },
    releaseResponses() {
      gate = null;
      open();
    },
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

function skeletonCount() {
  return document.querySelectorAll('[data-slot="skeleton"]').length;
}

describe("EnvGroupDetailPage editor across an EnvGroup poll (w1/m153 t003)", () => {
  it("keeps a draft row focused and typed, its rows unskeletoned and its controls enabled, across a refetch over cached data", async () => {
    const apollo = gatedClient();
    const user = userEvent.setup();
    renderDetail(apollo.client);

    const edit = await screen.findByRole("button", { name: "Edit" });
    expect(edit).toBeEnabled();
    await user.click(edit);
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Add variable" }),
    );

    const keyInputs = screen.getAllByRole("textbox", { name: "Key" });
    expect(keyInputs).toHaveLength(2);
    const [existingKey, draftKey] = keyInputs;
    await user.type(draftKey, "API_TOKEN");
    const draftValue = screen.getByRole("textbox", {
      name: "Value for API_TOKEN",
    });
    await user.type(draftValue, "s3cret");
    expect(draftValue).toHaveFocus();
    expect(
      screen.getByRole("button", { name: "Save and deploy" }),
    ).toBeEnabled();
    const skeletonsBefore = skeletonCount();
    const groupRequestsBefore = apollo.requests.filter(
      (name) => name === "EnvGroup",
    ).length;

    // A poll tick: refetch the group while its data is cached, and hold the
    // response so the in-flight state can be inspected.
    apollo.holdResponses();
    let refresh: Promise<unknown> = Promise.resolve();
    await act(async () => {
      refresh = apollo.client.refetchQueries({ include: [EnvGroupDocument] });
      await new Promise((resolve) => setTimeout(resolve, 20));
    });

    const expectDraftIntact = () => {
      expect(screen.getByRole("textbox", { name: "Value for API_TOKEN" })).toBe(
        draftValue,
      );
      expect(draftValue.isConnected).toBe(true);
      expect(draftValue).toHaveFocus();
      expect(draftValue).toHaveValue("s3cret");
      expect(screen.getAllByRole("textbox", { name: "Key" })).toEqual([
        existingKey,
        draftKey,
      ]);
      expect(existingKey).toHaveValue("FOO");
      expect(draftKey).toHaveValue("API_TOKEN");
      expect(skeletonCount()).toBe(skeletonsBefore);
      expect(
        screen.getByRole("button", { name: "Add variable" }),
      ).toBeEnabled();
      expect(
        screen.getByRole("button", { name: "Save and deploy" }),
      ).toBeEnabled();
    };

    // In flight: the refetch really went out, and nothing was swapped out.
    expect(
      apollo.requests.filter((name) => name === "EnvGroup").length,
    ).toBeGreaterThan(groupRequestsBefore);
    expectDraftIntact();

    await act(async () => {
      apollo.releaseResponses();
      await refresh;
      await new Promise((resolve) => setTimeout(resolve, 20));
    });

    // Settled: the same draft, still focused, still saveable.
    expectDraftIntact();
  });
});
