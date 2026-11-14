import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { print } from "graphql";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import {
  CreateWorkspaceDocument,
  FinalizeWorkspaceCreationDocument,
  WorkspacesDocument,
} from "@/graphql/definitions";
import { useWorkspaceIPAllowList } from "@/features/workspaces/hooks/use-workspace-ip-allow-list";
import { useSetWorkspaceIPAllowList } from "@/features/workspaces/hooks/use-set-workspace-ip-allow-list";

const toastSuccess = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: (...a: unknown[]) => toastSuccess(...a), error: vi.fn() },
}));

const SEEDED = [
  {
    __typename: "IPAllowListEntry",
    cidrBlock: "0.0.0.0/0",
    description: "Allow all (default)",
  },
  {
    __typename: "IPAllowListEntry",
    cidrBlock: "::/0",
    description: "Allow all (default)",
  },
];

type Reply =
  | { data: Record<string, unknown> }
  | { errors: Array<{ message: string; extensions?: Record<string, unknown> }> }
  | "network";

/**
 * Real Apollo with the app's cache and defaults; only the network is faked.
 * `WorkspaceIPAllowList` answers Render's seeded pair, and the mutation
 * answers `reply` (default: what bex-api saved, echoing the input).
 */
function fakeClient(reply?: Reply) {
  const operations: Array<{ name: string; variables: unknown }> = [];
  const link = new ApolloLink(
    (operation) =>
      new Observable((sub) => {
        operations.push({
          name: operation.operationName ?? "",
          variables: operation.variables,
        });
        setTimeout(() => {
          if (operation.operationName === "WorkspaceIPAllowList") {
            sub.next({
              data: {
                workspace: {
                  __typename: "Workspace",
                  id: (operation.variables as { id: string }).id,
                  ipAllowList: SEEDED,
                },
              },
            });
          } else if (reply === "network") {
            sub.error(new TypeError("Failed to fetch"));
            return;
          } else if (reply) {
            sub.next(reply);
          } else {
            const { id, ipAllowList } = operation.variables as {
              id: string;
              ipAllowList: Array<{ cidrBlock: string; description: string }>;
            };
            sub.next({
              data: {
                setWorkspaceIPAllowList: {
                  __typename: "Workspace",
                  id,
                  ipAllowList: ipAllowList.map((entry) => ({
                    __typename: "IPAllowListEntry",
                    ...entry,
                  })),
                },
              },
            });
          }
          sub.complete();
        }, 0);
      }),
  );
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
    defaultOptions: apolloDefaultOptions,
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return { wrapper, operations };
}

function useCardData() {
  return {
    read: useWorkspaceIPAllowList("tea-1"),
    rules: useSetWorkspaceIPAllowList(),
  };
}

async function mountLoaded(reply?: Reply) {
  const fake = fakeClient(reply);
  const view = renderHook(() => useCardData(), { wrapper: fake.wrapper });
  expect(view.result.current.read.loading).toBe(true);
  await waitFor(() => expect(view.result.current.read.loading).toBe(false));
  return { ...fake, ...view };
}

beforeEach(() => {
  toastSuccess.mockReset();
});

describe("workspace inbound IP rules hooks (w9/m388)", () => {
  // The polled workspace list must not pay for the rules: bex-api resolves
  // them only when selected, and only the settings card selects them.
  it("keeps ipAllowList out of the polled list and the create mutations", () => {
    for (const document of [
      WorkspacesDocument,
      CreateWorkspaceDocument,
      FinalizeWorkspaceCreationDocument,
    ]) {
      expect(print(document)).not.toContain("ipAllowList");
    }
  });

  it("reads the rules once and a save reaches the read without a refetch", async () => {
    const { result, operations } = await mountLoaded();
    expect(result.current.read.entries).toEqual([
      { cidrBlock: "0.0.0.0/0", description: "Allow all (default)" },
      { cidrBlock: "::/0", description: "Allow all (default)" },
    ]);

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.rules.save("tea-1", [
        { cidrBlock: " 203.0.113.0/24 ", description: " office " },
        { cidrBlock: "2001:db8::/32", description: "" },
      ]);
    });

    expect(ok).toBe(true);
    expect(
      operations.find((op) => op.name === "SetWorkspaceIPAllowList")?.variables,
    ).toEqual({
      id: "tea-1",
      ipAllowList: [
        { cidrBlock: "203.0.113.0/24", description: "office" },
        { cidrBlock: "2001:db8::/32", description: "" },
      ],
    });
    // A shorter list replaces the cached one outright (merge: false).
    expect(result.current.read.entries).toEqual([
      { cidrBlock: "203.0.113.0/24", description: "office" },
      { cidrBlock: "2001:db8::/32", description: "" },
    ]);
    expect(operations.map((op) => op.name)).toEqual([
      "WorkspaceIPAllowList",
      "SetWorkspaceIPAllowList",
    ]);
    expect(toastSuccess).toHaveBeenCalledWith("Inbound IP rules saved");
    expect(result.current.rules.refusal).toBeNull();
  });

  it("saves an empty list as is: deny-all is a real choice", async () => {
    const { result, operations } = await mountLoaded();
    await act(async () => {
      await result.current.rules.save("tea-1", []);
    });
    expect(
      operations.find((op) => op.name === "SetWorkspaceIPAllowList")?.variables,
    ).toEqual({ id: "tea-1", ipAllowList: [] });
    expect(result.current.read.entries).toEqual([]);
  });

  it("classifies a below-Scale refusal by its code and feature", async () => {
    const { result } = await mountLoaded({
      errors: [
        {
          message: "bad request: ipAllowList: reworded by the server",
          extensions: {
            code: "PLAN_FEATURE_REQUIRED",
            plan: "pro",
            feature: "inboundIPRules",
            requiredPlan: "scale",
            field: "ipAllowList",
          },
        },
      ],
    });
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.rules.save("tea-1", [
        { cidrBlock: "203.0.113.0/24", description: "" },
      ]);
    });
    expect(ok).toBe(false);
    expect(result.current.rules.refusal).toEqual({ kind: "plan" });
    // The stored list is untouched.
    expect(result.current.read.entries).toHaveLength(2);
  });

  it("classifies a non-Admin refusal as forbidden", async () => {
    const { result } = await mountLoaded({
      errors: [{ message: "forbidden", extensions: { code: "FORBIDDEN" } }],
    });
    await act(async () => {
      await result.current.rules.save("tea-1", []);
    });
    expect(result.current.rules.refusal).toEqual({ kind: "forbidden" });
  });

  it("relays any other refusal's reason, and the generic copy for a transport failure", async () => {
    const refused = await mountLoaded({
      errors: [
        {
          message: 'bad request: "10.0.0.0/33" is not a valid CIDR',
          extensions: { code: "BAD_REQUEST" },
        },
      ],
    });
    await act(async () => {
      await refused.result.current.rules.save("tea-1", []);
    });
    expect(refused.result.current.rules.refusal).toEqual({
      kind: "other",
      message: '"10.0.0.0/33" is not a valid CIDR',
    });
    refused.unmount();

    const offline = await mountLoaded("network");
    await act(async () => {
      await offline.result.current.rules.save("tea-1", []);
    });
    expect(offline.result.current.rules.refusal).toEqual({
      kind: "other",
      message: "Couldn't save the inbound IP rules. Please try again.",
    });
  });
});
