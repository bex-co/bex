import { describe, expect, it } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { BlueprintSyncsDocument } from "@/graphql/definitions";
import {
  BLUEPRINT_SYNCS_PAGE_SIZE,
  useBlueprintSyncs,
} from "../use-blueprint-syncs";

const ID = "exs-db136288mmqc73d4hpug";

function syncs(from: number, count: number) {
  return Array.from({ length: count }, (_, i) => ({
    __typename: "BlueprintSync",
    id: `exe-${String(from - i).padStart(3, "0")}`,
    commitId: "0123456789abcdef",
    commitMessage: "msg",
    state: "success",
    startedAt: "2026-11-26T10:00:00Z",
    completedAt: "2026-11-26T10:01:00Z",
    errorMessage: null,
    note: null,
  }));
}

// w3/m678: Syncs reads its first page, then older ones past the last sync
// shown, as Render's list loads more; the page after the last full one ends
// the list.
describe("useBlueprintSyncs", () => {
  it("appends older pages past the last sync shown", async () => {
    const first = syncs(40, BLUEPRINT_SYNCS_PAGE_SIZE);
    const link = new MockLink([
      {
        request: {
          query: BlueprintSyncsDocument,
          variables: { id: ID, limit: BLUEPRINT_SYNCS_PAGE_SIZE },
        },
        result: { data: { blueprintSyncs: first } },
      },
      {
        request: {
          query: BlueprintSyncsDocument,
          variables: {
            id: ID,
            cursor: first.at(-1)!.id,
            limit: BLUEPRINT_SYNCS_PAGE_SIZE,
          },
        },
        result: { data: { blueprintSyncs: syncs(20, 5) } },
      },
    ]);
    const client = new ApolloClient({ link, cache: new InMemoryCache() });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <ApolloProvider client={client}>{children}</ApolloProvider>
    );
    const { result } = renderHook(() => useBlueprintSyncs(ID), { wrapper });

    await waitFor(() =>
      expect(result.current.syncs).toHaveLength(BLUEPRINT_SYNCS_PAGE_SIZE),
    );
    expect(result.current.hasMore).toBe(true);

    act(() => result.current.loadMore());
    await waitFor(() =>
      expect(result.current.syncs).toHaveLength(BLUEPRINT_SYNCS_PAGE_SIZE + 5),
    );
    expect(result.current.syncs.at(-1)?.id).toBe("exe-016");
    expect(result.current.hasMore).toBe(false);
  });
});
