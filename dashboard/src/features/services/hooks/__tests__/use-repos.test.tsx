import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { Observable } from "rxjs";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { ReposDocument } from "@/graphql/definitions";
import type { Capabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useRepos } from "@/features/services/hooks/use-repos";
import { mockCapabilities } from "@/test/mocks/capabilities";

// bex-api lists a workspace's repositories for the roles that can use one
// (developer and up) and records any other role's request as a denial. The
// hook therefore sends nothing for a role it knows is refused, and reads as
// loading, not empty, while the role is still being resolved.

let capabilities: Capabilities;
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => capabilities,
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

const repo = {
  __typename: "Repo",
  id: 1,
  fullName: "acme/api",
  private: true,
  defaultBranch: "main",
  htmlUrl: "https://github.com/acme/api",
  cloneUrl: "https://github.com/acme/api.git",
  accountLogin: "acme",
  installationId: 42,
};

function harness() {
  const requests: string[] = [];
  const record = new ApolloLink((operation, forward) => {
    requests.push(operation.operationName ?? "");
    return new Observable<ApolloLink.Result>((observer) => {
      const subscription = forward(operation).subscribe(observer);
      return () => subscription.unsubscribe();
    });
  });
  const mocks = new MockLink(
    [
      {
        request: { query: ReposDocument, variables: { ownerId: "tea-1" } },
        result: { data: { repos: [repo] } },
        maxUsageCount: Number.POSITIVE_INFINITY,
      },
    ],
    { defaultOptions: { delay: 0 } },
  );
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: ApolloLink.from([record, mocks]),
    defaultOptions: apolloDefaultOptions,
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return { requests, wrapper };
}

beforeEach(() => {
  capabilities = mockCapabilities();
});

describe("useRepos", () => {
  it("lists the workspace's repositories for a role that can create", async () => {
    const { requests, wrapper } = harness();
    const { result } = renderHook(() => useRepos(), { wrapper });

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.repos).toHaveLength(1));
    expect(result.current.repos[0].fullName).toBe("acme/api");
    expect(result.current.loading).toBe(false);
    expect(requests).toEqual(["Repos"]);
  });

  it("sends nothing for a role bex-api would refuse, and reads as an empty list", async () => {
    capabilities = mockCapabilities({ role: "VIEWER", canCreate: false });
    const { requests, wrapper } = harness();
    const { result } = renderHook(() => useRepos(), { wrapper });

    expect(result.current.loading).toBe(false);
    expect(result.current.repos).toEqual([]);
    // Long enough for a request to have left, had one been made.
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(requests).toEqual([]);
    expect(result.current.error).toBeUndefined();
  });

  it("is loading, not empty, until the role is known, then lists", async () => {
    capabilities = mockCapabilities({
      role: null,
      canCreate: false,
      loading: true,
      loaded: false,
    });
    const { requests, wrapper } = harness();
    const { result, rerender } = renderHook(() => useRepos(), { wrapper });

    expect(result.current.loading).toBe(true);
    expect(result.current.repos).toEqual([]);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(requests).toEqual([]);

    capabilities = mockCapabilities();
    rerender();
    await waitFor(() => expect(result.current.repos).toHaveLength(1));
    expect(requests).toEqual(["Repos"]);
  });
});
