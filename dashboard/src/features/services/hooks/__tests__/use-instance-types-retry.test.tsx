import { describe, expect, it } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { useInstanceTypes } from "../use-instance-types";

/**
 * w4/m218 (a785): a cold InstanceTypes read that settles with a GraphQL
 * execution error left the create form with no plans and no way back short of
 * a reload — the retry link does not retry execution errors, and the hooks
 * exposed no refetch. Real Apollo client with the app's own defaults (no
 * network-status notifications); only the network is faked.
 */
function client(responses: Array<"error" | "ok">) {
  let reads = 0;
  const link = new ApolloLink(
    () =>
      new Observable((sub) => {
        const reply = responses[Math.min(reads, responses.length - 1)];
        reads += 1;
        setTimeout(() => {
          sub.next(
            reply === "error"
              ? { errors: [{ message: "catalog unavailable" }] }
              : {
                  data: {
                    instanceTypes: [
                      {
                        __typename: "InstanceType",
                        id: "free",
                        name: "Free",
                        cpu: "100m",
                        memory: "512Mi",
                        monthlyUsd: "0.00",
                      },
                      {
                        __typename: "InstanceType",
                        id: "starter",
                        name: "Starter",
                        cpu: "500m",
                        memory: "512Mi",
                        monthlyUsd: "4.90",
                      },
                    ],
                  },
                },
          );
          sub.complete();
        }, 0);
      }),
  );
  return {
    client: new ApolloClient({
      link,
      cache: new InMemoryCache(apolloCacheConfig),
      defaultOptions: apolloDefaultOptions,
    }),
    reads: () => reads,
  };
}

describe("useInstanceTypes retry (w4/m218)", () => {
  it("settles a failed read as an error, then a retry restores the catalog", async () => {
    const fake = client(["error", "ok"]);
    const wrapper = ({ children }: { children: ReactNode }) => (
      <ApolloProvider client={fake.client}>{children}</ApolloProvider>
    );
    const { result } = renderHook(() => useInstanceTypes(), { wrapper });
    await waitFor(() => expect(result.current.error).toBeDefined());
    expect(result.current.loading).toBe(false);
    expect(result.current.instanceTypes).toEqual([]);

    let first!: Promise<void>;
    act(() => {
      first = result.current.retry();
      void result.current.retry(); // a second click starts nothing
    });
    expect(result.current.retrying).toBe(true);
    await act(async () => {
      await first;
    });
    expect(fake.reads()).toBe(2);
    expect(result.current.retrying).toBe(false);
    expect(result.current.error).toBeUndefined();
    expect(result.current.instanceTypes.map((t) => t.id)).toEqual([
      "free",
      "starter",
    ]);
  });

  it("keeps the error when the retry fails too", async () => {
    const fake = client(["error", "error"]);
    const wrapper = ({ children }: { children: ReactNode }) => (
      <ApolloProvider client={fake.client}>{children}</ApolloProvider>
    );
    const { result } = renderHook(() => useInstanceTypes(), { wrapper });
    await waitFor(() => expect(result.current.error).toBeDefined());
    await act(async () => {
      await result.current.retry();
    });
    expect(result.current.retrying).toBe(false);
    expect(result.current.error).toBeDefined();
    expect(result.current.instanceTypes).toEqual([]);
  });
});
