import { describe, expect, it } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { AgentSkillVersionsDocument } from "@/graphql/definitions";
import { useAgentSkillVersions } from "../use-agent-skill-versions";

function version(n: number, current = false) {
  return {
    __typename: "AgentSkillVersion",
    version: n,
    shortId: `abc000${n}`,
    message: "Update skills/run/SKILL.md",
    createdBy: "usr-1",
    createdAt: "2026-11-06T10:00:00Z",
    current,
  };
}

function page(
  versions: ReturnType<typeof version>[],
  nextCursor: string | null,
) {
  return {
    data: {
      agentSkillVersions: {
        __typename: "AgentSkillVersionList",
        skillId: "skl-1",
        ownerId: "tea-1",
        versions,
        nextCursor,
      },
    },
  };
}

const first = {
  query: AgentSkillVersionsDocument,
  variables: { ownerId: "tea-1", id: "skl-1", limit: 20 },
};
const second = {
  query: AgentSkillVersionsDocument,
  variables: { ownerId: "tea-1", id: "skl-1", cursor: "4", limit: 20 },
};

function setup(mocks: MockLink.MockedResponse[]) {
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new MockLink(mocks),
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return renderHook(() => useAgentSkillVersions("tea-1", "skl-1"), {
    wrapper,
  });
}

describe("useAgentSkillVersions", () => {
  it("appends the next page by cursor and stops when there is none", async () => {
    const { result } = setup([
      {
        request: first,
        result: page([version(5, true), version(4)], "4"),
      },
      { request: second, result: page([version(3), version(2)], null) },
    ]);
    await waitFor(() => expect(result.current.versions).toHaveLength(2));
    expect(result.current.hasMore).toBe(true);
    await act(async () => {
      await result.current.loadMore();
    });
    expect(result.current.versions.map((v) => v.version)).toEqual([5, 4, 3, 2]);
    expect(result.current.versions.filter((v) => v.current)).toHaveLength(1);
    expect(result.current.hasMore).toBe(false);
  });

  it("keeps the loaded versions and flags a failed Load more, then retries the same page", async () => {
    const { result } = setup([
      { request: first, result: page([version(5, true), version(4)], "4") },
      { request: second, error: new Error("network") },
      { request: second, result: page([version(3)], null) },
    ]);
    await waitFor(() => expect(result.current.versions).toHaveLength(2));
    await act(async () => {
      await result.current.loadMore();
    });
    expect(result.current.loadMoreFailed).toBe(true);
    expect(result.current.versions).toHaveLength(2);
    expect(result.current.hasMore).toBe(true);
    await act(async () => {
      await result.current.loadMore();
    });
    expect(result.current.loadMoreFailed).toBe(false);
    expect(result.current.versions.map((v) => v.version)).toEqual([5, 4, 3]);
  });

  it("reports a failed first page as an error with no versions", async () => {
    const { result } = setup([{ request: first, error: new Error("boom") }]);
    await waitFor(() => expect(result.current.error).toBeDefined());
    expect(result.current.versions).toEqual([]);
    expect(result.current.hasMore).toBe(false);
  });
});
