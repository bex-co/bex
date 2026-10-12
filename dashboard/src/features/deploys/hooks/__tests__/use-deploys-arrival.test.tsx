import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { DeploysDocument } from "@/graphql/definitions";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { useDeploys } from "../use-deploys";

const toastError = vi.hoisted(() => vi.fn());
vi.mock("sonner", () => ({ toast: { error: toastError } }));

// w4/232: with the full history loaded (head [21…2] + tail [1]), a new deploy
// shifted page 1 to [22…3]. The old tail [1] didn't overlap it, so it stayed —
// and the list read as an exhausted 21 rows with deploy 2 missing between.
// These run the real cache and link: the head is a polled query, the tail is
// React state, and the defect lives in how the two are joined.

function deploy(n: number, status = "deactivated") {
  return {
    __typename: "Deploy" as const,
    id: `dep-${n}`,
    status,
    trigger: "api",
    image: "docker.io/library/busybox:1.37",
    rollbackOf: "",
    commitId: "",
    commitMessage: "",
    commitCreatedAt: null,
    createdAt: `2026-10-10T09:${String(n).padStart(2, "0")}:00Z`,
    updatedAt: null,
    startedAt: null,
    finishedAt: null,
    preDeployStatus: "",
    failureReason: "",
    cancelReason: "",
    stallReason: "",
  };
}

/** Deploys newest-first from `from` down to `to`; the newest is live. */
function range(from: number, to: number) {
  return Array.from({ length: from - to + 1 }, (_, i) =>
    deploy(from - i, i === 0 ? "live" : "deactivated"),
  );
}

function head(deploys: ReturnType<typeof deploy>[], status?: string[]) {
  return {
    request: {
      query: DeploysDocument,
      variables: { serviceId: "srv-1", status, limit: 20 },
    },
    result: { data: { deploys } },
  };
}

function older(
  cursor: string,
  deploys: ReturnType<typeof deploy>[],
  status?: string[],
) {
  return {
    request: {
      query: DeploysDocument,
      variables: { serviceId: "srv-1", status, cursor, limit: 20 },
    },
    result: { data: { deploys } },
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

const ids = (rows: { id: string }[]) => rows.map((r) => r.id);

beforeEach(() => {
  toastError.mockReset();
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

describe("useDeploys after a new deploy arrives (w4/232)", () => {
  it("re-offers Load more from the shifted head and recovers every deploy", async () => {
    const { result } = renderHook(() => useDeploys("srv-1", []), {
      wrapper: wrapperFor([
        head(range(21, 2)),
        older("dep-2", range(1, 1)),
        head(range(22, 3)),
        older("dep-3", range(2, 1)),
        head(range(23, 4)),
        older("dep-4", range(3, 1)),
        head(range(23, 4)),
      ]),
    });

    await waitFor(() => expect(result.current.deploys).toHaveLength(20));
    expect(result.current.hasMore).toBe(true);
    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.deploys).toHaveLength(21));
    expect(result.current.hasMore).toBe(false);

    // Deploy 22 arrives: the tail no longer continues [22…3], so the list is
    // the current head with Load more — never [22…3, 1] read as complete.
    await tickSettledPoll();
    await waitFor(() =>
      expect(ids(result.current.deploys)).toEqual(ids(range(22, 3))),
    );
    expect(result.current.hasMore).toBe(true);

    act(() => result.current.loadMore());
    await waitFor(() =>
      expect(ids(result.current.deploys)).toEqual(ids(range(22, 1))),
    );
    expect(result.current.hasMore).toBe(false);

    // Repeat with two loaded tail rows: deploy 23 arrives.
    await tickSettledPoll();
    await waitFor(() =>
      expect(ids(result.current.deploys)).toEqual(ids(range(23, 4))),
    );
    expect(result.current.hasMore).toBe(true);
    act(() => result.current.loadMore());
    await waitFor(() =>
      expect(ids(result.current.deploys)).toEqual(ids(range(23, 1))),
    );
    expect(result.current.hasMore).toBe(false);
    expect(toastError).not.toHaveBeenCalled();
  });

  it("keeps a valid tail across a status-only refresh", async () => {
    const settled = range(21, 2);
    const statusChanged = settled.map((d, i) =>
      i === 1 ? { ...d, status: "canceled" } : d,
    );
    const { result } = renderHook(() => useDeploys("srv-1", []), {
      wrapper: wrapperFor([
        head(settled),
        older("dep-2", range(1, 1)),
        head(statusChanged),
        head(statusChanged),
      ]),
    });

    await waitFor(() => expect(result.current.deploys).toHaveLength(20));
    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.deploys).toHaveLength(21));

    await tickSettledPoll();
    await waitFor(() =>
      expect(result.current.deploys[1]?.status).toBe("canceled"),
    );
    expect(ids(result.current.deploys)).toEqual(ids(range(21, 1)));
    expect(result.current.hasMore).toBe(false);
  });

  it("discards an older page that lands after the head shifted", async () => {
    const { result } = renderHook(() => useDeploys("srv-1", []), {
      wrapper: wrapperFor([
        head(range(21, 2)),
        { ...older("dep-2", range(1, 1)), delay: 40_000 },
        head(range(22, 3)),
        head(range(22, 3)),
        older("dep-3", range(2, 1)),
      ]),
    });

    await waitFor(() => expect(result.current.deploys).toHaveLength(20));
    act(() => result.current.loadMore());
    await tickSettledPoll();
    await waitFor(() =>
      expect(ids(result.current.deploys)).toEqual(ids(range(22, 3))),
    );
    // Now the page read past dep-2 answers; it must not reattach as [22…3, 1].
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    await waitFor(() => expect(result.current.loadingMore).toBe(false));
    expect(ids(result.current.deploys)).toEqual(ids(range(22, 3)));
    expect(result.current.hasMore).toBe(true);

    act(() => result.current.loadMore());
    await waitFor(() =>
      expect(ids(result.current.deploys)).toEqual(ids(range(22, 1))),
    );
    expect(result.current.hasMore).toBe(false);
  });

  it("an errored older-page read is announced, not taken as exhaustion", async () => {
    const { result } = renderHook(() => useDeploys("srv-1", []), {
      wrapper: wrapperFor([
        head(range(21, 2)),
        {
          request: {
            query: DeploysDocument,
            variables: { serviceId: "srv-1", cursor: "dep-2", limit: 20 },
          },
          result: { errors: [{ message: "store unavailable" }] },
        },
      ]),
    });

    await waitFor(() => expect(result.current.deploys).toHaveLength(20));
    act(() => result.current.loadMore());
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("Couldn't load older deploys."),
    );
    expect(ids(result.current.deploys)).toEqual(ids(range(21, 2)));
    expect(result.current.hasMore).toBe(true);
  });
});
