import {
  ApolloClient,
  ApolloLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import type { ServiceEventsQueryVariables } from "@/graphql/definitions";
import {
  useServiceEvents,
  type UseServiceEventsOptions,
} from "../use-service-events";

const now = Date.parse("2026-10-02T12:00:00Z");
const windowMs = 720 * 60 * 60 * 1000;
const bounds = {
  startTime: new Date(now - windowMs).toISOString(),
  endTime: new Date(now).toISOString(),
};
function event(id: string, offset: number) {
  return {
    __typename: "ServiceEvent" as const,
    id,
    cursor: id,
    type: "deploy_started",
    timestamp: new Date(now + offset).toISOString(),
    details: null,
  };
}
function fixture(
  options: Partial<UseServiceEventsOptions> = {},
  initialFailure = false,
) {
  const rows: Record<string, ReturnType<typeof event>[]> = {
    a: [
      event("a1", -1000),
      event("a2", -2000),
      event("a3", -3000),
      event("a4", -4000),
    ],
    b: [event("b1", -1000)],
  };
  const requests: ServiceEventsQueryVariables[] = [];
  let failure = initialFailure;
  let delay = false;
  const pending: (() => void)[] = [];
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    defaultOptions: apolloDefaultOptions,
    link: new ApolloLink(
      (operation) =>
        new Observable((observer) => {
          const vars = operation.variables as ServiceEventsQueryVariables;
          requests.push({ ...vars });
          const complete = () => {
            if (failure) {
              observer.error(new Error("events unavailable"));
              return;
            }
            const start = Date.parse(vars.startTime),
              end = Date.parse(vars.endTime);
            if (end - start > windowMs) {
              observer.error(new Error("range exceeds 720 hours"));
              return;
            }
            const matching = (rows[vars.serviceId] ?? [])
              .filter(
                (row) =>
                  Date.parse(row.timestamp) >= start &&
                  Date.parse(row.timestamp) <= end,
              )
              .sort(
                (a, b) => Date.parse(b.timestamp) - Date.parse(a.timestamp),
              );
            const offset = vars.cursor
              ? matching.findIndex((row) => row.cursor === vars.cursor) + 1
              : 0;
            observer.next({
              data: {
                serviceEvents: matching.slice(
                  offset,
                  offset + (vars.limit ?? 20),
                ),
              },
            });
            observer.complete();
          };
          if (delay) pending.push(complete);
          else complete();
        }),
    ),
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  const hook = renderHook(
    ({ id }) =>
      useServiceEvents(id, { ...bounds, limit: 2, live: true, ...options }),
    { wrapper, initialProps: { id: "a" } },
  );
  return {
    ...hook,
    rows,
    requests,
    pending,
    client,
    setFailure: (value: boolean) => {
      failure = value;
    },
    setDelay: (value: boolean) => {
      delay = value;
    },
  };
}
async function tick(ms = 1) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}
beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(now);
  vi.spyOn(document, "hidden", "get").mockReturnValue(false);
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("live service events", () => {
  it("retains older pages and cursor continuity while bridging bursts and delayed facts", async () => {
    const f = fixture();
    await tick();
    await act(() => f.result.current.loadMore());
    await tick();
    expect(f.result.current.events.map((e) => e.id)).toEqual([
      "a1",
      "a2",
      "a3",
      "a4",
    ]);
    f.rows.a.push(
      event("new1", 1000),
      event("new2", 2000),
      event("new3", 3000),
      event("late", -2500),
    );
    await tick(30_000);
    await tick();
    expect(f.result.current.events.map((e) => e.id)).toEqual([
      "new3",
      "new2",
      "new1",
      "a1",
      "a2",
      "late",
      "a3",
      "a4",
    ]);
    await act(() => f.result.current.loadMore());
    expect(f.requests.at(-1)).toMatchObject({ ...bounds, cursor: "a4" });
    expect(f.result.current.hasMore).toBe(false);
    expect(
      f.requests.every(
        (r) => Date.parse(r.endTime) - Date.parse(r.startTime) <= windowMs,
      ),
    ).toBe(true);
    f.client.stop();
  });
  it("skips hidden ticks, resumes with current bounds, and stops on unmount", async () => {
    const f = fixture();
    await tick();
    vi.spyOn(document, "hidden", "get").mockReturnValue(true);
    await tick(60_000);
    expect(f.requests).toHaveLength(1);
    vi.spyOn(document, "hidden", "get").mockReturnValue(false);
    f.rows.a.push(event("new", 65_000));
    await tick(30_000);
    await tick();
    expect(f.result.current.events[0].id).toBe("new");
    f.unmount();
    const count = f.requests.length;
    await tick(60_000);
    expect(f.requests).toHaveLength(count);
    f.client.stop();
  });
  it("keeps rows after a failed refresh and retries using fresh bounds", async () => {
    const f = fixture();
    await tick();
    f.setFailure(true);
    await tick(30_000);
    await tick();
    expect(f.result.current.error?.message).toContain("events unavailable");
    expect(f.result.current.events.map((e) => e.id)).toEqual(["a1", "a2"]);
    f.setFailure(false);
    f.rows.a.push(event("recovered", 40_000));
    vi.setSystemTime(now + 45_000);
    await act(() => f.result.current.refetch());
    await tick();
    expect(f.result.current.error).toBeUndefined();
    expect(f.result.current.events[0].id).toBe("recovered");
    expect(f.requests.at(-1)?.endTime).toBe(
      new Date(now + 45_000).toISOString(),
    );
    f.client.stop();
  });
  it("ignores an old service's pending older page and prevents duplicate requests", async () => {
    const f = fixture();
    await tick();
    f.setDelay(true);
    let older: Promise<void>;
    act(() => {
      older = f.result.current.loadMore();
      void f.result.current.loadMore();
    });
    expect(f.pending).toHaveLength(1);
    f.setDelay(false);
    f.rerender({ id: "b" });
    await tick();
    expect(f.result.current.events.map((e) => e.id)).toEqual(["b1"]);
    await act(async () => {
      f.pending[0]();
      await older;
    });
    await tick();
    expect(f.result.current.events.map((e) => e.id)).toEqual(["b1"]);
    expect(f.result.current.hasMore).toBe(false);
    f.client.stop();
  });
  it("ignores a pending live head after navigating away and back", async () => {
    const f = fixture();
    await tick();
    f.setDelay(true);
    await tick(30_000);
    expect(f.pending).toHaveLength(1);
    f.setDelay(false);
    f.rerender({ id: "b" });
    await tick();
    expect(f.result.current.events.map((e) => e.id)).toEqual(["b1"]);
    f.rerender({ id: "a" });
    await tick();
    f.rows.a.push(event("stale-response", 10_000));
    await act(async () => {
      f.pending[0]();
    });
    await tick();
    expect(f.result.current.events.map((e) => e.id)).toEqual(["a1", "a2"]);
    f.client.stop();
  });
  it("keeps explicit Metrics windows fixed and auto-paginates without a second timer", async () => {
    const f = fixture({ live: false, autoPaginate: true });
    await tick();
    await tick();
    await tick();
    expect(f.result.current.events).toHaveLength(4);
    const count = f.requests.length;
    await tick(60_000);
    expect(f.requests).toHaveLength(count);
    expect(
      f.requests.every(
        (r) => r.startTime === bounds.startTime && r.endTime === bounds.endTime,
      ),
    ).toBe(true);
    f.client.stop();
  });
  it("recovers an initial failed query with current bounds and usable history", async () => {
    const f = fixture({}, true);
    await tick();
    expect(f.result.current.error).toBeDefined();
    f.setFailure(false);
    vi.setSystemTime(now + 10_000);
    await act(() => f.result.current.refetch());
    await tick();
    expect(f.result.current.error).toBeUndefined();
    expect(f.result.current.events.length).toBeGreaterThan(0);
    await act(() => f.result.current.loadMore());
    await tick();
    expect(f.result.current.events.map((e) => e.id)).toEqual([
      "a1",
      "a2",
      "a3",
      "a4",
    ]);
    f.client.stop();
  });
  it("orders overlapping arrivals by instant across mixed RFC3339 precision", async () => {
    const f = fixture();
    await tick();
    f.rows.a.push(
      { ...event("fraction", 0), timestamp: "2026-10-02T12:00:00.999Z" },
      { ...event("whole", 0), timestamp: "2026-10-02T12:00:00Z" },
    );
    await tick(30_000);
    await tick();
    expect(f.result.current.events.slice(0, 2).map((e) => e.id)).toEqual([
      "fraction",
      "whole",
    ]);
    f.client.stop();
  });
});
