import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";

// test/setup.ts mocks this hook for every other test; this is its own.
vi.unmock("@/common/hooks/use-inbound-ip-levels");

const workspace = vi.hoisted(() => ({
  current: { id: "tea-1" } as { id: string } | null,
}));
vi.mock("@/features/workspaces/context", () => ({
  useWorkspace: () => ({ currentWorkspace: workspace.current }),
}));

import { useInboundIPLevels } from "@/common/hooks/use-inbound-ip-levels";

const entry = (cidrBlock: string) => ({
  __typename: "IPAllowListEntry",
  cidrBlock,
  description: `${cidrBlock} note`,
});

type Answer = Record<string, unknown> | "error";

/** Real Apollo with the app's cache; only the two reads are faked. */
function fakeClient(answers: { workspace: Answer; environment?: Answer }) {
  const operations: Array<{ name: string; variables: unknown }> = [];
  const link = new ApolloLink(
    (operation) =>
      new Observable((sub) => {
        const name = operation.operationName ?? "";
        operations.push({ name, variables: operation.variables });
        const answer =
          name === "WorkspaceIPAllowList"
            ? answers.workspace
            : answers.environment;
        setTimeout(() => {
          if (answer === "error" || answer === undefined) {
            sub.next({ errors: [{ message: "not found" }] });
          } else {
            sub.next({ data: answer });
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

const workspaceRead = {
  workspace: {
    __typename: "Workspace",
    id: "tea-1",
    ipAllowList: [entry("10.0.0.0/8")],
  },
};
const environmentRead = {
  environment: {
    __typename: "Environment",
    id: "evm-1",
    projectId: "prj-1",
    ipAllowListEntries: [entry("10.1.0.0/16")],
  },
};

describe("useInboundIPLevels", () => {
  it("reads the workspace's rules alone for a resource outside any environment", async () => {
    workspace.current = { id: "tea-1" };
    const { wrapper, operations } = fakeClient({ workspace: workspaceRead });
    const { result } = renderHook(() => useInboundIPLevels(null), {
      wrapper,
    });
    expect(result.current).toEqual({ state: "loading" });
    await waitFor(() => expect(result.current.state).toBe("ready"));
    expect(result.current).toEqual({
      state: "ready",
      workspace: [{ cidrBlock: "10.0.0.0/8", description: "10.0.0.0/8 note" }],
    });
    expect(operations.map((operation) => operation.name)).toEqual([
      "WorkspaceIPAllowList",
    ]);
  });

  it("adds the environment's rules and a link to where they are edited", async () => {
    workspace.current = { id: "tea-1" };
    const { wrapper, operations } = fakeClient({
      workspace: workspaceRead,
      environment: environmentRead,
    });
    const { result } = renderHook(() => useInboundIPLevels("evm-1"), {
      wrapper,
    });
    await waitFor(() => expect(result.current.state).toBe("ready"));
    expect(result.current).toMatchObject({
      environment: {
        entries: [
          { cidrBlock: "10.1.0.0/16", description: "10.1.0.0/16 note" },
        ],
        href: "/project/prj-1/environment/evm-1/settings#networking",
      },
    });
    expect(operations).toContainEqual({
      name: "EnvironmentIpAllowList",
      variables: { id: "evm-1" },
    });
  });

  it("claims nothing before the workspace is known", () => {
    workspace.current = null;
    const { wrapper, operations } = fakeClient({ workspace: workspaceRead });
    const { result } = renderHook(() => useInboundIPLevels(null), {
      wrapper,
    });
    expect(result.current).toEqual({ state: "loading" });
    expect(operations).toEqual([]);
  });

  it.each([
    ["the workspace's", { workspace: "error" as const }],
    [
      "the environment's",
      { workspace: workspaceRead, environment: "error" as const },
    ],
  ])("is an error when %s rules cannot be read", async (_, answers) => {
    workspace.current = { id: "tea-1" };
    const { wrapper } = fakeClient(answers);
    const { result } = renderHook(() => useInboundIPLevels("evm-1"), {
      wrapper,
    });
    await waitFor(() => expect(result.current.state).toBe("error"));
  });
});
