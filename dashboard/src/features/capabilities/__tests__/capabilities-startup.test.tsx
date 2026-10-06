import type { ReactNode } from "react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApolloCsrClient } from "@/common/apollo/factory.client";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { WorkspaceProvider } from "@/features/workspaces/context";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { WORKSPACE_SELECTION_KEY } from "@/features/workspaces/lib/selection";
import { CapabilitiesProvider } from "@/features/capabilities/context/capabilities-provider";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { resetAccessGenerationForTests } from "@/features/capabilities/lib/access-generation";
import { CAPABILITY_ACTIONS } from "@/features/capabilities/lib/capability-policy";
import { WorkspacesDocument } from "@/graphql/definitions";

vi.unmock("@/features/capabilities/hooks/use-capabilities");

const { mockFetch, navigate } = vi.hoisted(() => ({
  mockFetch: vi.fn<typeof fetch>(),
  navigate: vi.fn(),
}));

vi.mock("@/common/hooks/use-is-authenticated", () => ({
  useIsAuthenticated: () => true,
}));
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => navigate,
}));
vi.mock("@/common/apollo/auth-redirect", () => ({
  handleUnauthenticated: vi.fn(),
  handleEmailVerificationRequired: vi.fn(),
}));

const workspaces = ["tea-first", "tea-second"].map((id) => ({
  __typename: "Workspace" as const,
  id,
  name: id,
  plan: "hobby",
  role: "admin",
  createdAt: null,
}));

interface Operation {
  operationName: string;
  variables: { ownerId?: string };
}

const dispatched: Operation[] = [];
let client: ApolloClient;
let capabilityOutcome: "allowed" | "denied" | "unavailable";

function capabilityResult() {
  const allowed = capabilityOutcome === "allowed";
  return {
    data: {
      viewerCapabilities: {
        __typename: "ViewerCapabilities",
        role: allowed ? "ADMIN" : "VIEWER",
        canView: allowed,
        canViewLogs: allowed,
        canOperate: allowed,
        canCreate: allowed,
        canViewSensitive: allowed,
        canManageKeys: allowed,
        canManage: allowed,
        canManageBilling: allowed,
        fresh: true,
        grants: CAPABILITY_ACTIONS.map((action) => ({
          __typename: "CapabilityGrant",
          action,
          outcome: capabilityOutcome,
          reason: allowed ? null : "insufficient_permission",
        })),
      },
    },
  };
}

beforeEach(() => {
  vi.useFakeTimers();
  // Make retry schedules reproducible. Otherwise jitter can accidentally
  // separate operations that shared a canceled batch and hide that defect.
  vi.spyOn(Math, "random").mockReturnValue(0.5);
  vi.stubGlobal("fetch", mockFetch);
  resetAccessGenerationForTests();
  navigate.mockReset();
  mockFetch.mockReset();
  dispatched.length = 0;
  capabilityOutcome = "allowed";
  document.cookie = `${WORKSPACE_SELECTION_KEY}=; Max-Age=0; Path=/`;
  mockFetch.mockImplementation(async (_input, options) => {
    const body = JSON.parse(String(options?.body)) as Operation | Operation[];
    const operations = Array.isArray(body) ? body : [body];
    const signal = options?.signal;
    // Native fetch rejects an already-aborted signal before a request reaches
    // the server. Preserve that boundary; a mock client.query cannot expose it.
    signal?.throwIfAborted();
    dispatched.push(...operations);
    const results = operations.map((operation) => {
      if (operation.operationName === "Workspaces") {
        return { data: { workspaces } };
      }
      if (capabilityOutcome === "unavailable") {
        throw new TypeError("The network request failed");
      }
      return capabilityResult();
    });
    return new Response(
      JSON.stringify(Array.isArray(body) ? results : results[0]),
      {
        headers: { "content-type": "application/json" },
      },
    );
  });
  // Reuse the production transport (auth errors, retry, batching), with a fresh
  // cache/query manager per test. Do not reproduce its configuration here: the
  // regression needs to exercise changes to the actual CSR client.
  client = new ApolloClient({
    link: createApolloCsrClient().link,
    cache: new InMemoryCache(apolloCacheConfig),
    defaultOptions: apolloDefaultOptions,
  });
});

afterEach(async () => {
  cleanup();
  client.stop();
  await vi.advanceTimersByTimeAsync(0);
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function mount(initialWorkspaceId: string | null = null) {
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <ApolloProvider client={client}>
        <WorkspaceProvider initialWorkspaceId={initialWorkspaceId}>
          <CapabilitiesProvider>{children}</CapabilitiesProvider>
        </WorkspaceProvider>
      </ApolloProvider>
    );
  }
  return renderHook(
    () => ({ workspace: useWorkspace(), capabilities: useCapabilities() }),
    { wrapper: Wrapper },
  );
}

async function settleBeforePoll() {
  // Includes the full transport retry budget, but never reaches the resource
  // poll. Success must come from initialization or the requested refresh.
  await act(async () => {
    await vi.advanceTimersByTimeAsync(5_000);
  });
}

describe("capability initialization through the browser transport", () => {
  it("settles a retained workspace selection before its first poll (navigation control)", async () => {
    const { result } = mount("tea-first");
    await settleBeforePoll();

    expect(result.current.capabilities).toMatchObject({
      canCreate: true,
      loaded: true,
      unavailable: false,
    });
    expect(
      dispatched.some((op) => op.operationName === "ViewerCapabilities"),
    ).toBe(true);
  });

  it("settles a cold missing-cookie fallback before its first poll", async () => {
    const { result } = mount();
    expect(result.current.workspace.currentWorkspaceId).toBeNull();
    expect(result.current.capabilities.canCreate).toBe(false);
    await settleBeforePoll();

    expect(result.current.workspace.currentWorkspaceId).toBe("tea-first");
    expect(result.current.capabilities).toMatchObject({
      canCreate: true,
      loaded: true,
      unavailable: false,
    });
    expect(
      dispatched.some((op) => op.operationName === "ViewerCapabilities"),
    ).toBe(true);
  });

  it("settles the new workspace when selection changes before the first batch dispatch", async () => {
    client.writeQuery({ query: WorkspacesDocument, data: { workspaces } });
    const { result } = mount("tea-first");
    act(() => result.current.workspace.setCurrentWorkspaceId("tea-second"));
    expect(result.current.capabilities.canCreate).toBe(false);
    await settleBeforePoll();

    expect(result.current.workspace.currentWorkspaceId).toBe("tea-second");
    expect(result.current.capabilities).toMatchObject({
      canCreate: true,
      loaded: true,
      unavailable: false,
    });
    expect(dispatched.some((op) => op.variables.ownerId === "tea-second")).toBe(
      true,
    );
  });

  it("keeps a transport failure distinct from denial and recovers on explicit refresh", async () => {
    capabilityOutcome = "unavailable";
    const { result } = mount("tea-first");
    await settleBeforePoll();
    expect(result.current.capabilities.unavailable).toBe(true);
    expect(result.current.capabilities.loaded).toBe(false);
    expect(result.current.capabilities.denied("can_create")).toBe(false);

    capabilityOutcome = "denied";
    act(() => void result.current.capabilities.refresh());
    await settleBeforePoll();
    expect(result.current.capabilities.unavailable).toBe(false);
    expect(result.current.capabilities.loaded).toBe(true);
    expect(result.current.capabilities.denied("can_create")).toBe(true);
    expect(result.current.capabilities.canCreate).toBe(false);

    capabilityOutcome = "allowed";
    act(() => void result.current.capabilities.refresh());
    await settleBeforePoll();
    expect(result.current.capabilities.loaded).toBe(true);
    expect(result.current.capabilities.canCreate).toBe(true);
  });
});
