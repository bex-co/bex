import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import {
  BlueprintSyncsDocument,
  SyncBlueprintDocument,
} from "@/graphql/definitions";
import { RESOURCE_POLL_INTERVAL_MS } from "@/common/lib/polling";
import { useBlueprintSyncs } from "../use-blueprint-syncs";
import { useSyncBlueprint } from "../use-sync-blueprint";

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), warning: vi.fn(), error: vi.fn() },
}));

const COMMIT = "c827ec0b0000000000000000000000000000beef";

function run(id: string, startedAt: string) {
  return {
    __typename: "BlueprintSync",
    id,
    commitId: COMMIT,
    state: "success",
    startedAt,
    completedAt: startedAt,
    errorMessage: null,
    note: null,
  };
}

const syncsQuery = {
  query: BlueprintSyncsDocument,
  variables: { id: "blp-1", ownerId: "tea-1", limit: 20 },
};

// w4/m138: a sync the dashboard just ran must appear in Sync History without a
// reload. Real client, real hooks; only the network is scripted: the history
// read, the sync, then the history read the sync's refetch must issue.
describe("Blueprint sync → Sync History", () => {
  it("re-reads history after a successful sync, so the new run is the top row", async () => {
    const older = run("bsr-older", "2026-09-25T05:00:00Z");
    const newest = run("bsr-newest", "2026-09-25T05:33:04Z");
    const link = new MockLink([
      { request: syncsQuery, result: { data: { blueprintSyncs: [older] } } },
      {
        request: {
          query: SyncBlueprintDocument,
          variables: {
            id: "blp-1",
            ownerId: "tea-1",
            commitId: COMMIT,
            path: "render.yaml",
            repo: "https://github.com/bex-co/bex",
          },
        },
        result: {
          data: {
            syncBlueprint: {
              __typename: "SyncBlueprintResult",
              blueprint: {
                __typename: "Blueprint",
                id: "blp-1",
                name: "qa-bp",
                status: "in_sync",
                lastSync: newest.startedAt,
                updatedAt: newest.startedAt,
              },
              services: [],
              databases: [],
              detachedResources: [],
            },
          },
        },
      },
      {
        request: syncsQuery,
        result: { data: { blueprintSyncs: [newest, older] } },
      },
    ]);
    const client = new ApolloClient({ link, cache: new InMemoryCache() });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <ApolloProvider client={client}>{children}</ApolloProvider>
    );
    const { result } = renderHook(
      () => ({ history: useBlueprintSyncs("blp-1"), sync: useSyncBlueprint() }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.history.syncs).toHaveLength(1));

    await act(async () => {
      await result.current.sync.sync("blp-1", {
        reviewed: {
          commitId: COMMIT,
          path: "render.yaml",
          repo: "https://github.com/bex-co/bex",
        },
      });
    });

    await waitFor(() =>
      expect(result.current.history.syncs.map((s) => s.id)).toEqual([
        "bsr-newest",
        "bsr-older",
      ]),
    );
  });

  it("polls history on the shared cadence for runs recorded elsewhere", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const pushed = run("bsr-autosync", "2026-09-25T06:00:00Z");
      const link = new MockLink([
        { request: syncsQuery, result: { data: { blueprintSyncs: [] } } },
        {
          request: syncsQuery,
          result: { data: { blueprintSyncs: [pushed] } },
        },
      ]);
      const client = new ApolloClient({ link, cache: new InMemoryCache() });
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ApolloProvider client={client}>{children}</ApolloProvider>
      );
      const { result } = renderHook(() => useBlueprintSyncs("blp-1"), {
        wrapper,
      });
      await waitFor(() => expect(result.current.loading).toBe(false));
      expect(result.current.syncs).toEqual([]);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(RESOURCE_POLL_INTERVAL_MS);
      });
      await waitFor(() =>
        expect(result.current.syncs.map((s) => s.id)).toEqual(["bsr-autosync"]),
      );
    } finally {
      vi.useRealTimers();
    }
  });
});
