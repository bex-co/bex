import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import {
  act,
  cleanup,
  render,
  renderHook,
  screen,
  waitFor,
} from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { GraphQLError } from "graphql";
import { Observable } from "rxjs";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { apolloCacheConfig } from "@/common/apollo/cache";
import {
  useServerActions,
  useDeployActions,
} from "@/features/capabilities/hooks/use-resource-actions";
import { useBoundActionConfirm } from "@/features/capabilities/hooks/use-bound-action-confirm";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  bumpAccessGeneration,
  getAccessGeneration,
  resetAccessGenerationForTests,
} from "@/features/capabilities/lib/access-generation";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { Capabilities } from "@/features/capabilities/hooks/use-capabilities";
import { decisionForSelectedRollback } from "@/features/capabilities/lib/resource-actions";
import { DeploysListPage } from "@/features/deploys/components/deploys-list-page";
import type { DeployRow } from "@/features/deploys/hooks/use-deploys";

let workspaceId: string | null = "tea-first";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: workspaceId }),
}));
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));
let deployRows: DeployRow[] = [];
vi.mock("@/features/deploys/hooks/use-deploys", () => ({
  useDeploys: () => ({
    deploys: deployRows,
    loading: false,
    loadingMore: false,
    hasMore: false,
    loadMore: vi.fn(),
  }),
}));
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({ service: { repo: null } }),
}));

type Pending = {
  answer: (response: ApolloLink.Result) => void;
  fail: (error: Error) => void;
  field: "serverActions" | "deployActions" | "viewerCapabilities";
};

let client: ApolloClient;
let mode: "allowed" | "hold" | "network-error" | "partial-error";
let pending: Pending[];
let capabilityOverrides: Partial<Capabilities>;
let capabilityOutcome: "allowed" | "denied" | "unavailable";
let errorField: Pending["field"] | null;
let requests: { name: string; variables: Record<string, unknown> }[];

function response(field: Pending["field"]): ApolloLink.Result {
  if (field === "viewerCapabilities")
    return {
      data: {
        viewerCapabilities: {
          __typename: "ViewerCapabilities",
          role: "ADMIN",
          fresh: true,
          canView: true,
          canViewLogs: true,
          canOperate: true,
          canCreate: capabilityOutcome === "allowed",
          canViewSensitive: true,
          canManageKeys: true,
          canManage: true,
          canManageBilling: true,
          grants: [
            { action: "can_create", outcome: capabilityOutcome, reason: null },
          ],
        },
      },
    };
  return {
    data: {
      [field]: (field === "serverActions"
        ? ["suspend", "resume", "restart"]
        : ["deploy", "cancel_deploy", "rollback"]
      ).map((action) => ({
        __typename: "ActionDecision",
        action,
        outcome: "allowed",
        reason: null,
        precondition: null,
      })),
    },
  };
}

function Wrapper({ children }: { children: ReactNode }) {
  return <ApolloProvider client={client}>{children}</ApolloProvider>;
}

beforeEach(() => {
  workspaceId = "tea-first";
  resetAccessGenerationForTests();
  capabilityOverrides = {};
  capabilityOutcome = "allowed";
  errorField = null;
  requests = [];
  vi.mocked(useCapabilities).mockImplementation(() =>
    mockCapabilities({
      generation: getAccessGeneration(),
      ...capabilityOverrides,
    }),
  );
  mode = "allowed";
  pending = [];
  client = new ApolloClient({
    cache: new InMemoryCache(apolloCacheConfig),
    link: new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          requests.push({
            name: operation.operationName ?? "",
            variables: operation.variables,
          });
          const field =
            operation.operationName === "ServerActions"
              ? "serverActions"
              : operation.operationName === "ViewerCapabilities"
                ? "viewerCapabilities"
                : "deployActions";
          const request: Pending = {
            field,
            answer: (result) => {
              observer.next(result);
              observer.complete();
            },
            fail: (error) => observer.error(error),
          };
          if (mode === "hold") {
            pending.push(request);
          } else {
            const currentMode = mode;
            queueMicrotask(() => {
              if (currentMode === "network-error") {
                request.fail(new Error("permission transport unavailable"));
              } else {
                request.answer({
                  ...response(field),
                  ...(currentMode === "partial-error" &&
                  (!errorField || errorField === field)
                    ? {
                        errors: [
                          new GraphQLError("permission evaluation failed"),
                        ],
                      }
                    : {}),
                });
              }
            });
          }
        }),
    ),
  });
});

afterEach(() => {
  cleanup();
  client.stop();
});

describe("resource action projection freshness", () => {
  it("shares one service permission request across a full deploy-history page", async () => {
    deployRows = Array.from({ length: 20 }, (_, index) => ({
      id: `dep-${index}`,
      status: "deactivated",
      trigger: "api",
      image: "",
      rollbackOf: "",
      commitId: "",
      commitMessage: "",
      commitCreatedAt: null,
      createdAt: null,
      updatedAt: null,
      startedAt: null,
      finishedAt: null,
      preDeployStatus: "",
      failureReason: "",
      cancelReason: "",
      stallReason: "",
    }));
    const root = createRootRoute();
    const route = createRoute({
      getParentRoute: () => root,
      path: "/services/$serviceId/deploys",
      component: () => <DeploysListPage serviceId="srv-first" />,
    });
    const router = createRouter({
      routeTree: root.addChildren([route]),
      history: createMemoryHistory({
        initialEntries: ["/services/srv-first/deploys"],
      }),
      context: { client: {} as never, session: null },
    });
    render(
      <Wrapper>
        <RouterProvider router={router} />
      </Wrapper>,
    );
    await waitFor(() => {
      const buttons = screen.getAllByRole("button", {
        name: /^Roll back to dep-/,
      });
      expect(buttons).toHaveLength(20);
      for (const button of buttons) expect(button).toBeEnabled();
    });
    expect(requests).toEqual([
      {
        name: "DeployActions",
        variables: { serviceId: "srv-first", ownerId: "tea-first" },
      },
    ]);
  });
  it("preserves receipt time on rerender and refreshes after a shared capability receipt", async () => {
    const { result, rerender } = renderHook(
      () => useServerActions("srv-first"),
      { wrapper: Wrapper },
    );
    await waitFor(() => expect(result.current.status).toBe("ready"));
    const snapshot =
      result.current.status === "ready" ? result.current.snapshot : null;
    rerender();
    expect(result.current.status === "ready" && result.current.snapshot).toBe(
      snapshot,
    );
    expect(requests).toEqual([
      {
        name: "ServerActions",
        variables: { id: "srv-first", ownerId: "tea-first" },
      },
    ]);
    mode = "hold";
    capabilityOverrides = { checkedAt: 2 };
    await act(async () => rerender());
    expect(result.current.status).toBe("checking");
    expect(pending).toHaveLength(1);
    await act(async () => pending[0].answer(response("serverActions")));
    expect(result.current.status).toBe("ready");
  });

  it.each(["stale", "unavailable"] as const)(
    "drops an allowed projection when the shared capability check becomes %s",
    async (failure) => {
      const { result, rerender } = renderHook(
        () => useServerActions("srv-first"),
        { wrapper: Wrapper },
      );
      await waitFor(() => expect(result.current.status).toBe("ready"));
      capabilityOverrides = { loaded: false, checkedAt: null, [failure]: true };
      rerender();
      expect(result.current.status).toBe("unavailable");
    },
  );
  it.each([
    ["server", useServerActions],
    ["deploy", useDeployActions],
  ] as const)(
    "drops a warm allowed %s projection when refresh fails",
    async (_name, useActions) => {
      const { result } = renderHook(() => useActions("srv-first"), {
        wrapper: Wrapper,
      });
      await waitFor(() => expect(result.current.status).toBe("ready"));

      mode = "network-error";
      await act(async () => {
        await result.current.refresh();
      });
      expect(result.current.status).toBe("unavailable");
    },
  );

  it("does not grant from a partial GraphQL response that also reports an error", async () => {
    const { result } = renderHook(() => useServerActions("srv-first"), {
      wrapper: Wrapper,
    });
    await waitFor(() => expect(result.current.status).toBe("ready"));
    mode = "partial-error";
    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.status).toBe("unavailable");
  });

  it.each(["workspace", "generation"])(
    "invalidates cached decisions immediately after a %s change",
    async (change) => {
      const { result, rerender } = renderHook(
        () => useServerActions("srv-first"),
        { wrapper: Wrapper },
      );
      await waitFor(() => expect(result.current.status).toBe("ready"));
      mode = "hold";
      await act(async () => {
        if (change === "workspace") workspaceId = "tea-second";
        bumpAccessGeneration();
        rerender();
      });
      expect(result.current.status).not.toBe("ready");
    },
  );

  it("does not bind an obsolete in-flight response to the next access generation", async () => {
    mode = "hold";
    const { result, rerender } = renderHook(
      () => useDeployActions("srv-first"),
      { wrapper: Wrapper },
    );
    await waitFor(() => expect(pending.length).toBeGreaterThan(0));
    const obsolete = pending.splice(0);
    act(() => {
      bumpAccessGeneration();
      rerender();
    });
    await act(async () => {
      for (const request of obsolete) request.answer(response(request.field));
    });
    expect(result.current.status).not.toBe("ready");
  });
});

describe("confirmation recheck leases", () => {
  it("preserves selected-target rollback eligibility when rechecking the service summary", async () => {
    mode = "hold";
    const { result } = renderHook(
      () =>
        useBoundActionConfirm({
          resourceId: "srv-first",
          deployId: "dep-first",
          adjustDecision: (_action, decision) =>
            decisionForSelectedRollback(decision),
        }),
      { wrapper: Wrapper },
    );
    let completion!: ReturnType<typeof result.current.recheckBeforeDispatch>;
    act(() => {
      const binding = result.current.openConfirm("rollback");
      completion = result.current.recheckBeforeDispatch(binding);
    });
    await waitFor(() => expect(pending).toHaveLength(1));
    let allowed = false;
    await act(async () => {
      pending[0].answer({
        data: {
          deployActions: [
            {
              __typename: "ActionDecision",
              action: "rollback",
              outcome: "allowed",
              reason: null,
              precondition: "no_eligible_rollback_target",
            },
          ],
        },
      });
      allowed = (await completion).ok;
    });
    expect(allowed).toBe(true);
  });
  it("checks an explicitly captured binding before the open state has rerendered", async () => {
    const { result } = renderHook(
      () => useBoundActionConfirm({ resourceId: "srv-first" }),
      { wrapper: Wrapper },
    );
    let allowed = false;
    await act(async () => {
      const binding = result.current.openConfirm("deploy");
      allowed = (await result.current.recheckBeforeDispatch(binding)).ok;
    });
    expect(allowed).toBe(true);
    expect(requests).toEqual([
      {
        name: "DeployActions",
        variables: { serviceId: "srv-first", ownerId: "tea-first" },
      },
    ]);
  });

  it.each(["allowed", "denied", "unavailable"] as const)(
    "requires a fresh can_create grant for a pinned deploy: %s",
    async (outcome) => {
      capabilityOutcome = outcome;
      const { result } = renderHook(
        () =>
          useBoundActionConfirm({
            resourceId: "srv-first",
            requiredCapability: "can_create",
          }),
        { wrapper: Wrapper },
      );
      let allowed = false;
      await act(async () => {
        const binding = result.current.openConfirm("deploy");
        allowed = (await result.current.recheckBeforeDispatch(binding)).ok;
      });
      expect(allowed).toBe(outcome === "allowed");
      expect(requests).toContainEqual({
        name: "ViewerCapabilities",
        variables: { ownerId: "tea-first", fresh: true },
      });
    },
  );

  it.each(["cancel", "unmount", "replace"])(
    "refuses an in-flight recheck after %s",
    async (change) => {
      mode = "hold";
      const { result, unmount } = renderHook(
        () => useBoundActionConfirm({ resourceId: "srv-first" }),
        { wrapper: Wrapper },
      );
      let completion!: ReturnType<typeof result.current.recheckBeforeDispatch>;
      act(() => {
        const binding = result.current.openConfirm("suspend");
        completion = result.current.recheckBeforeDispatch(binding);
      });
      await waitFor(() => expect(pending).toHaveLength(1));
      act(() => {
        if (change === "unmount") unmount();
        else if (change === "replace") result.current.openConfirm("resume");
        else result.current.clearConfirm();
      });
      let allowed = true;
      await act(async () => {
        pending[0].answer(response("serverActions"));
        allowed = (await completion).ok;
      });
      expect(allowed).toBe(false);
      if (change === "replace")
        expect(result.current.pending?.action).toBe("resume");
    },
  );

  it("refuses partial GraphQL errors in the extra fresh capability check", async () => {
    mode = "partial-error";
    errorField = "viewerCapabilities";
    const { result } = renderHook(
      () =>
        useBoundActionConfirm({
          resourceId: "srv-first",
          requiredCapability: "can_create",
        }),
      { wrapper: Wrapper },
    );
    let allowed = true;
    await act(async () => {
      const binding = result.current.openConfirm("deploy");
      allowed = (await result.current.recheckBeforeDispatch(binding)).ok;
    });
    expect(
      requests.some((request) => request.name === "ViewerCapabilities"),
    ).toBe(true);
    expect(allowed).toBe(false);
    expect(result.current.pending).toBeNull();
  });

  it("keeps an open binding through an ordinary poll while shared grants are fresh", async () => {
    const { result, rerender } = renderHook(
      () => useBoundActionConfirm({ resourceId: "srv-first" }),
      { wrapper: Wrapper },
    );
    act(() => result.current.openConfirm("rollback"));
    const binding = result.current.pending;
    capabilityOverrides = { loading: true, loaded: true };
    rerender();
    expect(result.current.pending).toBe(binding);
    expect(result.current.isBindingCurrent(binding)).toBe(true);
  });

  it.each(["stale", "unavailable", "checking"] as const)(
    "keeps same-context intent but refuses dispatch while access is %s",
    async (gap) => {
      const { result, rerender } = renderHook(
        () => useBoundActionConfirm({ resourceId: "srv-first" }),
        { wrapper: Wrapper },
      );
      act(() => result.current.openConfirm("suspend"));
      const binding = result.current.pending;
      capabilityOverrides = {
        loaded: false,
        checkedAt: null,
        ...(gap === "checking" ? { loading: true } : { [gap]: true }),
      };
      rerender();
      expect(result.current.pending).toBe(binding);
      expect(result.current.isIntentCurrent(binding)).toBe(true);
      expect(result.current.isBindingCurrent(binding)).toBe(false);
      expect(result.current.blockedReason).toBe(
        gap === "unavailable"
          ? "Permissions could not be refreshed. Try again — this is not a role change."
          : "Checking whether you can perform this action…",
      );
      let allowed = true;
      await act(async () => {
        allowed = (await result.current.recheckBeforeDispatch(binding)).ok;
      });
      expect(allowed).toBe(false);
      expect(requests).toEqual([]);
      expect(result.current.pending).toBe(binding);

      capabilityOverrides = {};
      rerender();
      expect(result.current.blockedReason).toBeUndefined();
      expect(result.current.isBindingCurrent(binding)).toBe(true);
    },
  );
  it.each(["workspace", "generation", "resource", "deploy"])(
    "refuses dispatch if the %s changes while the recheck is pending",
    async (change) => {
      mode = "hold";
      const { result, rerender } = renderHook(
        ({ resourceId, deployId }) =>
          useBoundActionConfirm({ resourceId, deployId }),
        {
          initialProps: { resourceId: "srv-first", deployId: "dep-first" },
          wrapper: Wrapper,
        },
      );
      act(() => result.current.openConfirm("rollback"));
      expect(result.current.pending).not.toBeNull();
      const dispatch = vi.fn();
      let completion!: Promise<void>;
      act(() => {
        completion = result.current.recheckBeforeDispatch().then(({ ok }) => {
          if (ok) dispatch();
        });
      });
      await waitFor(() => expect(pending.length).toBe(1));
      act(() => {
        if (change === "workspace") workspaceId = "tea-second";
        if (change === "workspace" || change === "generation")
          bumpAccessGeneration();
        rerender({
          resourceId: change === "resource" ? "srv-second" : "srv-first",
          deployId: change === "deploy" ? "dep-second" : "dep-first",
        });
      });
      expect(result.current.pending).toBeNull();
      await act(async () => {
        pending[0].answer(response(pending[0].field));
        await completion;
      });
      expect(dispatch).not.toHaveBeenCalled();
    },
  );

  it("refuses a recheck with both allowed rows and a GraphQL error", async () => {
    mode = "partial-error";
    const { result } = renderHook(
      () => useBoundActionConfirm({ resourceId: "srv-first" }),
      { wrapper: Wrapper },
    );
    act(() => result.current.openConfirm("suspend"));
    let allowed = false;
    await act(async () => {
      allowed = (await result.current.recheckBeforeDispatch()).ok;
    });
    expect(allowed).toBe(false);
    expect(result.current.pending).toBeNull();
  });

  it("keeps the valid same-context confirmation usable", async () => {
    const { result } = renderHook(
      () =>
        useBoundActionConfirm({
          resourceId: "srv-first",
          deployId: "dep-first",
        }),
      { wrapper: Wrapper },
    );
    act(() => result.current.openConfirm("rollback"));
    let checked:
      | Awaited<ReturnType<typeof result.current.recheckBeforeDispatch>>
      | undefined;
    await act(async () => {
      checked = await result.current.recheckBeforeDispatch();
    });
    expect(checked).toMatchObject({
      ok: true,
      binding: {
        resourceId: "srv-first",
        deployId: "dep-first",
        workspaceId: "tea-first",
      },
    });
  });
});
