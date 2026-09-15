import { describe, it, expect } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider, useLazyQuery, useQuery } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { EnvironmentsDocument } from "@/graphql/definitions";
import { useEnvironments } from "@/features/environments/hooks/use-environments";

// w1/m153: a background poll or refetch over cached data must not re-announce
// `loading`. Apollo 4 does by default, and every `loading ? <Skeleton/>` gate
// then unmounted the page under it every 30 s — at filing time the environment
// settings dialog closed mid-edit and the repo search lost focus.

const environment = {
  __typename: "Environment",
  id: "env-1",
  projectId: "prj-1",
  name: "staging",
  ownerId: "tea-1",
  createdAt: "2026-09-14T00:00:00Z",
  serviceIds: [],
  databaseIds: [],
  keyValueIds: [],
  envGroupIds: [],
  protectedStatus: "unprotected",
  networkIsolationEnabled: false,
  ipAllowList: [],
  ipAllowListEntries: [],
};

function client(defaultOptions?: typeof apolloDefaultOptions) {
  return new ApolloClient({
    cache: new InMemoryCache(),
    link: new MockLink([
      {
        request: {
          query: EnvironmentsDocument,
          variables: { projectId: "prj-1" },
        },
        result: { data: { environments: [environment] } },
        maxUsageCount: Number.POSITIVE_INFINITY,
      },
    ]),
    ...(defaultOptions ? { defaultOptions } : {}),
  });
}

function wrapperFor(apollo: ApolloClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <ApolloProvider client={apollo}>{children}</ApolloProvider>;
  };
}

/**
 * Every raw `loading` Apollo reports — through first data, then through a
 * refetch — for a query shaped like the dashboard's polled hooks
 * (cache-and-network with a poll interval). Raw `useQuery`, not a hook, so the
 * client default is what is measured, not a hook's own first-load guard.
 */
async function loadingAcrossRefetch(apollo: ApolloClient) {
  const firstLoad: boolean[] = [];
  const refresh: boolean[] = [];
  let phase = firstLoad;
  const { result } = renderHook(
    () => {
      const query = useQuery(EnvironmentsDocument, {
        variables: { projectId: "prj-1" },
        fetchPolicy: "cache-and-network",
        pollInterval: 30_000,
      });
      phase.push(query.loading);
      return query;
    },
    { wrapper: wrapperFor(apollo) },
  );
  await waitFor(() =>
    expect(result.current.data?.environments).toHaveLength(1),
  );
  phase = refresh;
  await act(async () => {
    await result.current.refetch();
    await new Promise((resolve) => setTimeout(resolve, 20));
  });
  return { firstLoad, refresh, result };
}

describe("apolloDefaultOptions (w1/m153)", () => {
  it("a refetch over cached data never reports loading, and the data stays", async () => {
    const { refresh, result } = await loadingAcrossRefetch(
      client(apolloDefaultOptions),
    );

    expect(refresh).not.toContain(true);
    expect(result.current.data?.environments?.map((env) => env?.id)).toEqual([
      "env-1",
    ]);
  });

  // The hook layer on top: useEnvironments keeps its card-facing `loading`
  // false across the refetch and keeps the environments it already mapped.
  it("useEnvironments keeps its environments and reports no loading across a refetch", async () => {
    const seen: boolean[] = [];
    const { result } = renderHook(
      () => {
        const view = useEnvironments("prj-1");
        seen.push(view.loading);
        return view;
      },
      { wrapper: wrapperFor(client(apolloDefaultOptions)) },
    );
    await waitFor(() => expect(result.current.environments).toHaveLength(1));
    seen.length = 0;
    await act(async () => {
      await result.current.refetch();
      await new Promise((resolve) => setTimeout(resolve, 20));
    });

    expect(seen).not.toContain(true);
    expect(result.current.environments.map((env) => env.id)).toEqual(["env-1"]);
  });

  it("the first load still reports loading — the one time a skeleton belongs", async () => {
    const { firstLoad } = await loadingAcrossRefetch(
      client(apolloDefaultOptions),
    );

    expect(firstLoad[0]).toBe(true);
  });

  // Control: the harness detects the bug. Apollo 4's own default re-announces
  // loading on the same refetch, which is what unmounted stateful UI.
  it("without the defaults, the same refetch reports loading (the filed bug)", async () => {
    const { refresh } = await loadingAcrossRefetch(client());

    expect(refresh).toContain(true);
  });

  // A lazy query learns of its own request only through the network-status
  // emission, so under the dashboard default it must opt in to show loading.
  it("a lazy query reports loading only when it opts in", async () => {
    const seenFor = async (notifyOnNetworkStatusChange?: boolean) => {
      const seen: boolean[] = [];
      const { result } = renderHook(
        () => {
          const [run, state] = useLazyQuery(EnvironmentsDocument, {
            ...(notifyOnNetworkStatusChange === undefined
              ? {}
              : { notifyOnNetworkStatusChange }),
          });
          seen.push(state.loading);
          return { run, state };
        },
        { wrapper: wrapperFor(client(apolloDefaultOptions)) },
      );
      await act(async () => {
        await result.current.run({ variables: { projectId: "prj-1" } });
        await new Promise((resolve) => setTimeout(resolve, 20));
      });
      return seen;
    };

    expect(await seenFor(true)).toContain(true);
    expect(await seenFor()).not.toContain(true);
  });

  it("a query that opts in still reports refreshing", async () => {
    const apollo = client(apolloDefaultOptions);
    const seen: boolean[] = [];
    const { result } = renderHook(
      () => {
        const query = useQuery(EnvironmentsDocument, {
          variables: { projectId: "prj-1" },
          notifyOnNetworkStatusChange: true,
        });
        seen.push(query.loading);
        return query;
      },
      { wrapper: wrapperFor(apollo) },
    );
    await waitFor(() =>
      expect(result.current.data?.environments).toHaveLength(1),
    );
    seen.length = 0;
    await act(async () => {
      await result.current.refetch();
      await new Promise((resolve) => setTimeout(resolve, 20));
    });

    expect(seen).toContain(true);
  });
});
