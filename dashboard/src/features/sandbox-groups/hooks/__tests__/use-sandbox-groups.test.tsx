import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { mockWorkspaceContext } from "@/test/mocks/workspace";
import { useSandboxGroups } from "@/features/sandbox-groups/hooks/use-sandbox-groups";

vi.mock("@/features/workspaces/context/hooks", () =>
  mockWorkspaceContext("tea-1"),
);

function withApi(answer: Record<string, unknown>) {
  const client = new ApolloClient({
    link: new ApolloLink(
      () =>
        new Observable((sub) => {
          setTimeout(() => {
            sub.next(answer);
            sub.complete();
          });
        }),
    ),
    cache: new InMemoryCache(apolloCacheConfig),
    defaultOptions: apolloDefaultOptions,
  });
  return ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
}

// w22/m603: the project page reads the groups for its environment cards. A
// bex-api without sandboxes has no such field, and that must read as no
// groups, never as a broken page.
describe("useSandboxGroups", () => {
  it("answers no groups, done loading, when bex-api has no sandboxes", async () => {
    const { result } = renderHook(() => useSandboxGroups(), {
      wrapper: withApi({
        errors: [
          {
            message: 'Cannot query field "sandboxGroups" on type "Query".',
            extensions: { code: "GRAPHQL_VALIDATION_FAILED" },
          },
        ],
      }),
    });

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.groups).toEqual([]);
    expect(result.current.error).toBeDefined();
  });

  it("answers the workspace's groups", async () => {
    const group = {
      __typename: "SandboxGroup",
      id: "sbg-1",
      name: "agents",
      ownerId: "tea-1",
      region: "oregon",
      isDefault: true,
      environmentId: "evm-staging",
      concurrencyLimit: 1000,
      createdAt: null,
      updatedAt: null,
    };
    const { result } = renderHook(() => useSandboxGroups(), {
      wrapper: withApi({ data: { sandboxGroups: [group, null] } }),
    });

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.groups).toEqual([group]);
  });
});
