import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { RESOURCE_POLL_INTERVAL_MS } from "@/common/lib/polling";
import { useMetricsFilterValues } from "../use-metrics-filter-values";

/**
 * w4/m224 (a875): discovery was read once per mount, so a status code the App
 * started returning never became selectable on an open Metrics page. Real
 * Apollo with the app's defaults; only the network and the clock are faked.
 */
type Reply = { status: string[]; instances?: string[] } | "error";

function fakeClient(replies: Record<string, Reply[]>) {
  const reads: string[] = [];
  const link = new ApolloLink(
    (operation) =>
      new Observable((sub) => {
        const { query } = operation.variables as {
          query: { filters: Array<{ values: string[] }> };
        };
        const resource = query.filters[0].values[0];
        const script = replies[resource];
        const seen = reads.filter((r) => r === resource).length;
        const reply = script[Math.min(seen, script.length - 1)];
        reads.push(resource);
        setTimeout(() => {
          if (reply === "error") {
            sub.next({ data: null, errors: [{ message: "metrics down" }] });
          } else {
            sub.next({
              data: {
                metricsFilters: {
                  __typename: "MetricsFiltersResult",
                  values: [
                    {
                      __typename: "MetricsFilterValues",
                      field: "INSTANCE",
                      values: reply.instances ?? ["pod-a"],
                    },
                    {
                      __typename: "MetricsFilterValues",
                      field: "STATUS_CODE",
                      values: reply.status,
                    },
                  ],
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
  return { wrapper, reads };
}

// Both production callers on one page: Network owns the refresh, Application
// observes the same document.
function useBothCards(resource: string) {
  return {
    status: useMetricsFilterValues(resource, "STATUS_CODE", { poll: true }),
    instances: useMetricsFilterValues(resource, "INSTANCE"),
  };
}

const tick = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const setHidden = (hidden: boolean) =>
  Object.defineProperty(document, "hidden", {
    configurable: true,
    value: hidden,
  });

describe("useMetricsFilterValues refresh (w4/m224)", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
    setHidden(false);
  });

  it("offers a newly returned code on the next poll, to both cards", async () => {
    const fake = fakeClient({
      web: [
        { status: ["200"], instances: ["pod-a"] },
        { status: ["200", "204"], instances: ["pod-a", "pod-b"] },
      ],
    });
    const { result } = renderHook(() => useBothCards("web"), {
      wrapper: fake.wrapper,
    });
    await tick(10);
    expect(result.current).toEqual({ status: ["200"], instances: ["pod-a"] });

    await tick(RESOURCE_POLL_INTERVAL_MS + 10);
    // The Application card observes the owner's refresh of the shared entry.
    expect(result.current).toEqual({
      status: ["200", "204"],
      instances: ["pod-a", "pod-b"],
    });
    expect(fake.reads).toEqual(["web", "web"]);
  });

  it("does not poll from the observing caller", async () => {
    const fake = fakeClient({
      web: [{ status: ["200"] }, { status: ["204"] }],
    });
    const { result } = renderHook(
      () => useMetricsFilterValues("web", "INSTANCE"),
      { wrapper: fake.wrapper },
    );
    await tick(RESOURCE_POLL_INTERVAL_MS * 2 + 10);
    expect(result.current).toEqual(["pod-a"]);
    expect(fake.reads).toEqual(["web"]);
  });

  it("keeps the last answer through a failed poll", async () => {
    const fake = fakeClient({
      web: [
        { status: ["200", "404"] },
        "error",
        { status: ["200", "404", "409"] },
      ],
    });
    const { result } = renderHook(() => useBothCards("web"), {
      wrapper: fake.wrapper,
    });
    await tick(10);
    await tick(RESOURCE_POLL_INTERVAL_MS + 10);
    expect(fake.reads).toHaveLength(2);
    expect(result.current.status).toEqual(["200", "404"]);
    await tick(RESOURCE_POLL_INTERVAL_MS + 10);
    expect(result.current.status).toEqual(["200", "404", "409"]);
  });

  it("never shows another resource's answer as current", async () => {
    const fake = fakeClient({
      web: [{ status: ["200", "500"] }],
      api: ["error"],
    });
    const { result, rerender } = renderHook(
      ({ resource }) => useBothCards(resource),
      { wrapper: fake.wrapper, initialProps: { resource: "web" } },
    );
    await tick(10);
    expect(result.current.status).toEqual(["200", "500"]);
    rerender({ resource: "api" });
    expect(result.current.status).toEqual([]);
    await tick(10);
    expect(result.current.status).toEqual([]);
  });

  it("skips polls while the tab is hidden and resumes when visible", async () => {
    const fake = fakeClient({ web: [{ status: ["200"] }] });
    renderHook(() => useBothCards("web"), { wrapper: fake.wrapper });
    await tick(10);
    setHidden(true);
    await tick(RESOURCE_POLL_INTERVAL_MS * 2 + 10);
    expect(fake.reads).toEqual(["web"]);
    setHidden(false);
    await tick(RESOURCE_POLL_INTERVAL_MS + 10);
    expect(fake.reads).toEqual(["web", "web"]);
  });
});
