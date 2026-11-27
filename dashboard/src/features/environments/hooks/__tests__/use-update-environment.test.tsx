import { describe, expect, it } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { useEnvironments } from "../use-environments";
import { useUpdateEnvironment } from "../use-update-environment";

/**
 * A settings card's save, with an Environments read (a poll, the page's own
 * first read) already in flight that answers after the save with the old
 * value. The save must still be what the list ends on. Real Apollo client and
 * cache; the fake server's list answers carry the state at request time and
 * can be held.
 */
function setup() {
  const server = {
    isolated: false,
    hold: false,
    held: [] as Array<() => void>,
    log: [] as string[],
  };
  const environment = () => ({
    __typename: "Environment",
    id: "evm-dev",
    projectId: "prj-1",
    name: "dev",
    ownerId: "tea-1",
    createdAt: null,
    serviceIds: [],
    databaseIds: [],
    keyValueIds: [],
    envGroupIds: [],
    protectedStatus: "unprotected",
    networkIsolationEnabled: server.isolated,
    ipAllowList: [],
    ipAllowListEntries: [],
    ipAllowListProxiedDomains: [],
  });
  const link = new ApolloLink(
    (op) =>
      new Observable((sub) => {
        const name = op.operationName ?? "";
        server.log.push(name);
        const send = (result: Record<string, unknown>) =>
          setTimeout(() => {
            sub.next(result);
            sub.complete();
          }, 0);
        if (name === "UpdateEnvironment") {
          const variables = op.variables as {
            networkIsolationEnabled?: boolean;
          };
          if (variables.networkIsolationEnabled !== undefined) {
            server.isolated = variables.networkIsolationEnabled;
          }
          send({ data: { updateEnvironment: environment() } });
          return;
        }
        if (name === "Environments") {
          const result = { data: { environments: [environment()] } };
          if (server.hold) server.held.push(() => send(result));
          else send(result);
          return;
        }
        sub.error(new Error(`unexpected operation ${name}`));
      }),
  );
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  const hook = renderHook(
    () => ({
      list: useEnvironments("prj-1"),
      save: useUpdateEnvironment(),
    }),
    { wrapper },
  );
  return { server, client, ...hook };
}

describe("useUpdateEnvironment", () => {
  it("sends only the patched field and shows the answer at once", async () => {
    const { result, server } = setup();
    await waitFor(() =>
      expect(result.current.list.environments).toHaveLength(1),
    );

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.save.update("evm-dev", {
        networkIsolationEnabled: true,
      });
    });

    expect(outcome).toEqual({ ok: true });
    expect(result.current.list.environments[0]?.networkIsolationEnabled).toBe(
      true,
    );
    expect(server.log).toContain("UpdateEnvironment");
  });

  it("ends on the save even when an older list read answers after it", async () => {
    const { result, server, client } = setup();
    await waitFor(() =>
      expect(result.current.list.environments).toHaveLength(1),
    );
    // A read starts before the save and is held past it.
    server.hold = true;
    void client
      .refetchQueries({ include: ["Environments"] })
      .catch(() => undefined);
    await waitFor(() => expect(server.held).toHaveLength(1));

    await act(async () => {
      await result.current.save.update("evm-dev", {
        networkIsolationEnabled: true,
      });
    });
    await act(async () => {
      server.hold = false;
      server.held.splice(0).forEach((send) => send());
      await new Promise((resolve) => setTimeout(resolve, 30));
    });

    await waitFor(() =>
      expect(result.current.list.environments[0]?.networkIsolationEnabled).toBe(
        true,
      ),
    );
  });
});
