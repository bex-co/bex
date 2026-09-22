import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { CronJobRunsDocument } from "@/graphql/definitions";
import { apolloCacheConfig } from "@/common/apollo/cache";
import {
  useCronRuns,
  mergeCronRunPages,
} from "@/features/services/hooks/use-cron-runs";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({ t: (k: string) => k }),
}));

// w4/138: the operator creates scheduled runs with no dashboard mutation to
// hang a refetch on, and Apollo's Server poll only rewrites CronRun ENTITIES
// already referenced by this list — nothing inserts a NEW reference into the
// root cronJobRuns array. So a mounted Recent Runs table showed its original
// rows indefinitely while the API already reported newer successful runs; only
// a reload added them. These exercise the real cache and link, because the
// defect lives exactly in that entity-versus-array-membership distinction.

function cronRun(n: number, status = "successful") {
  return {
    __typename: "CronRun" as const,
    id: `crr-${n}`,
    status,
    startedAt: `2026-09-22T05:${String(40 + n).padStart(2, "0")}:00Z`,
    finishedAt:
      status === "successful"
        ? `2026-09-22T05:${String(40 + n).padStart(2, "0")}:05Z`
        : null,
  };
}

type Run = ReturnType<typeof cronRun>;

function firstPage(serviceId: string, runs: Run[]) {
  return {
    request: {
      query: CronJobRunsDocument,
      variables: { serviceId, limit: 5 },
    },
    result: { data: { cronJobRuns: runs } },
  };
}

function olderPage(serviceId: string, cursor: string, runs: Run[]) {
  return {
    request: {
      query: CronJobRunsDocument,
      variables: { serviceId, cursor, limit: 5 },
    },
    result: { data: { cronJobRuns: runs } },
  };
}

function wrapperFor(mocks: object[]) {
  const client = new ApolloClient({
    link: new MockLink(mocks as never) as ApolloLink,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <ApolloProvider client={client}>{children}</ApolloProvider>;
  };
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
});

afterEach(() => {
  vi.useRealTimers();
});

/** Advance past one settled (30s) poll interval. */
async function tickSettledPoll() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(31_000);
  });
}

describe("useCronRuns freshness (w4/138)", () => {
  it("discovers a scheduled run that arrives while the page stays mounted", async () => {
    const baseline = [cronRun(2), cronRun(1)];
    const withArrival = [cronRun(3), cronRun(2), cronRun(1)];
    const { result } = renderHook(() => useCronRuns("srv-1"), {
      wrapper: wrapperFor([
        firstPage("srv-1", baseline),
        firstPage("srv-1", withArrival),
        firstPage("srv-1", withArrival),
      ]),
    });

    await waitFor(() =>
      expect(result.current.runs.map((r) => r.id)).toEqual(["crr-2", "crr-1"]),
    );

    // No mutation, no reload — just the baseline tick on a SETTLED history,
    // which is precisely the state a new scheduled run arrives into.
    await tickSettledPoll();
    await waitFor(() =>
      expect(result.current.runs.map((r) => r.id)).toEqual([
        "crr-3",
        "crr-2",
        "crr-1",
      ]),
    );
  });

  it("carries a pending run through to its terminal status", async () => {
    const pending = [cronRun(2, "running"), cronRun(1)];
    const done = [cronRun(2), cronRun(1)];
    const { result } = renderHook(() => useCronRuns("srv-1"), {
      wrapper: wrapperFor([
        firstPage("srv-1", pending),
        firstPage("srv-1", done),
        firstPage("srv-1", done),
      ]),
    });

    await waitFor(() => expect(result.current.hasActiveRun).toBe(true));

    await tickSettledPoll();
    await waitFor(() => expect(result.current.hasActiveRun).toBe(false));
    expect(result.current.runs[0]?.status).toBe("successful");
  });

  it("keeps loaded older rows when a poll refreshes the first page", async () => {
    // The regression the note insisted on: adding a pollInterval alone would
    // rewrite the merged cache entry with a fresh 5-row first page, silently
    // dropping the rows Load more had fetched.
    const head = [cronRun(9), cronRun(8), cronRun(7), cronRun(6), cronRun(5)];
    const tail = [cronRun(4), cronRun(3), cronRun(2), cronRun(1)];
    const { result } = renderHook(() => useCronRuns("srv-1"), {
      wrapper: wrapperFor([
        firstPage("srv-1", head),
        olderPage("srv-1", "crr-5", tail),
        firstPage("srv-1", head),
        firstPage("srv-1", head),
      ]),
    });

    await waitFor(() => expect(result.current.runs).toHaveLength(5));
    expect(result.current.hasMore).toBe(true);

    await act(async () => {
      await result.current.loadMore();
    });
    await waitFor(() => expect(result.current.runs).toHaveLength(9));
    // A short last page means there is nothing further to load.
    expect(result.current.hasMore).toBe(false);

    await tickSettledPoll();
    // Still nine rows, in order, after the refresh.
    expect(result.current.runs.map((r) => r.id)).toEqual([
      "crr-9",
      "crr-8",
      "crr-7",
      "crr-6",
      "crr-5",
      "crr-4",
      "crr-3",
      "crr-2",
      "crr-1",
    ]);
  });

  it("drops a tail the refreshed head no longer joins, and restores Load more", async () => {
    // One arrival moves page 1 from [9…5] to [10…6], so the tail read from
    // crr-5 no longer joins the head — crr-5 itself belongs to neither.
    // Concatenating anyway would hide a run in the MIDDLE of the history with
    // hasMore already false, so it could not be recovered. Dropping the tail
    // shows a correct first page and makes Load more work again.
    const head = [cronRun(9), cronRun(8), cronRun(7), cronRun(6), cronRun(5)];
    const tail = [cronRun(4), cronRun(3), cronRun(2), cronRun(1)];
    const shifted = [
      cronRun(10),
      cronRun(9),
      cronRun(8),
      cronRun(7),
      cronRun(6),
    ];
    const { result } = renderHook(() => useCronRuns("srv-1"), {
      wrapper: wrapperFor([
        firstPage("srv-1", head),
        olderPage("srv-1", "crr-5", tail),
        firstPage("srv-1", shifted),
        firstPage("srv-1", shifted),
      ]),
    });

    await waitFor(() => expect(result.current.runs).toHaveLength(5));
    await act(async () => {
      await result.current.loadMore();
    });
    await waitFor(() => expect(result.current.runs).toHaveLength(9));
    expect(result.current.hasMore).toBe(false);

    await tickSettledPoll();

    await waitFor(() =>
      expect(result.current.runs.map((r) => r.id)).toEqual([
        "crr-10",
        "crr-9",
        "crr-8",
        "crr-7",
        "crr-6",
      ]),
    );
    // No hole, no duplicate, and the tail is reachable again.
    expect(result.current.hasMore).toBe(true);
  });

  it("keeps the rows it has when a refresh fails", async () => {
    const baseline = [cronRun(2), cronRun(1)];
    const { result } = renderHook(() => useCronRuns("srv-1"), {
      wrapper: wrapperFor([
        firstPage("srv-1", baseline),
        {
          request: {
            query: CronJobRunsDocument,
            variables: { serviceId: "srv-1", limit: 5 },
          },
          error: new Error("ERR_CONNECTION_CLOSED"),
        },
      ]),
    });

    await waitFor(() => expect(result.current.runs).toHaveLength(2));

    await tickSettledPoll();
    // A transient failure must not blank the history or flash a skeleton.
    expect(result.current.runs.map((r) => r.id)).toEqual(["crr-2", "crr-1"]);
    expect(result.current.loading).toBe(false);
  });

  it("does not carry one service's loaded tail onto another service", async () => {
    const aHead = [cronRun(9), cronRun(8), cronRun(7), cronRun(6), cronRun(5)];
    const aTail = [cronRun(4), cronRun(3)];
    const bHead = [cronRun(2)];
    const { result, rerender } = renderHook(
      ({ id }: { id: string }) => useCronRuns(id),
      {
        initialProps: { id: "srv-1" },
        wrapper: wrapperFor([
          firstPage("srv-1", aHead),
          olderPage("srv-1", "crr-5", aTail),
          firstPage("srv-2", bHead),
          firstPage("srv-2", bHead),
        ]),
      },
    );

    await waitFor(() => expect(result.current.runs).toHaveLength(5));
    await act(async () => {
      await result.current.loadMore();
    });
    await waitFor(() => expect(result.current.runs).toHaveLength(7));

    rerender({ id: "srv-2" });
    await waitFor(() =>
      expect(result.current.runs.map((r) => r.id)).toEqual(["crr-2"]),
    );
  });
});

describe("mergeCronRunPages", () => {
  const a = {
    id: "a",
    status: "successful",
    startedAt: null,
    finishedAt: null,
  };
  const b = {
    id: "b",
    status: "successful",
    startedAt: null,
    finishedAt: null,
  };
  const c = {
    id: "c",
    status: "successful",
    startedAt: null,
    finishedAt: null,
  };

  it("appends a disjoint tail in order", () => {
    expect(mergeCronRunPages([a], [b, c])).toEqual({
      rows: [a, b, c],
      dropAppended: false,
    });
  });

  it("drops the tail when the first page overlaps it", () => {
    expect(mergeCronRunPages([a, b], [b, c])).toEqual({
      rows: [a, b],
      dropAppended: true,
    });
  });

  it("drops the tail when its anchor has fallen off the first page", () => {
    // Anchor "z" is not on page 1, so head and tail no longer join up.
    expect(mergeCronRunPages([a], [b, c], "z")).toEqual({
      rows: [a],
      dropAppended: true,
    });
    // Anchor still present: contiguous, so the tail is kept.
    expect(mergeCronRunPages([a], [b, c], "a")).toEqual({
      rows: [a, b, c],
      dropAppended: false,
    });
  });
});
