import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { toast } from "sonner";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { useSetEnvironmentDatabases } from "../use-set-environment-databases";
import { useEnvironments } from "../use-environments";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

/**
 * w4/325 (a720): moving a Postgres database between environments completed,
 * but an Environments read that started before the write answered after it,
 * with the source still holding the database — its row and count stayed until
 * a later poll, after both the bulk Move and the Manage dialog's Save. The
 * same ordered publication Key Value got in w4/263 now covers databases. Real Apollo client and cache; only the network is faked, as a
 * server whose list answers carry the membership AT REQUEST TIME and can be
 * held.
 */
const DB = "dpg-db8oscsdp0tc73djg5ng";

interface FakeServer {
  membership: Record<string, string[]>;
  log: string[];
  hold: boolean;
  releaseAfterWrite: boolean;
  failListsAfterWrite: number;
  refusal?: string;
  written: boolean;
  held: Array<() => void>;
}

let server: FakeServer;

function environment(name: string, databaseIds: string[]) {
  return {
    __typename: "Environment",
    id: `evm-${name}`,
    projectId: "prj-1",
    name,
    ownerId: "tea-1",
    createdAt: "2026-10-12T00:58:00Z",
    serviceIds: [],
    databaseIds,
    keyValueIds: [],
    envGroupIds: [],
    protectedStatus: "unprotected",
    networkIsolationEnabled: false,
    ipAllowList: [],
    ipAllowListEntries: [],
    ipAllowListProxiedDomains: [],
  };
}

function release() {
  server.hold = false;
  server.held.splice(0).forEach((send) => send());
}

const link = new ApolloLink(
  (op) =>
    new Observable((sub) => {
      const opName = op.operationName ?? "";
      server.log.push(`→${opName}`);
      const send = (result: Record<string, unknown>) =>
        setTimeout(() => {
          server.log.push(`←${opName}`);
          sub.next(result);
          sub.complete();
        }, 0);

      if (opName === "SetEnvironmentDatabases") {
        if (server.refusal) {
          send({ errors: [{ message: server.refusal }] });
          return;
        }
        const { id, databaseIds } = op.variables as {
          id: string;
          databaseIds: string[];
        };
        const name = id.replace("evm-", "");
        // Joining one environment leaves every other one in the project.
        for (const other of Object.keys(server.membership)) {
          server.membership[other] = server.membership[other].filter(
            (db) => !databaseIds.includes(db),
          );
        }
        server.membership[name] = databaseIds;
        server.written = true;
        send({
          data: { setEnvironmentDatabases: environment(name, databaseIds) },
        });
        if (server.releaseAfterWrite) setTimeout(release, 20);
        return;
      }
      if (opName === "Projects") {
        send({ data: { projects: [] } });
        return;
      }
      if (opName === "Environments") {
        if (server.written && server.failListsAfterWrite > 0) {
          server.failListsAfterWrite -= 1;
          setTimeout(() => sub.error(new Error("ERR_CONNECTION_CLOSED")), 0);
          return;
        }
        const result = {
          data: {
            environments: Object.entries(server.membership).map(([n, ids]) =>
              environment(n, [...ids]),
            ),
          },
        };
        if (server.hold) server.held.push(() => send(result));
        else send(result);
        return;
      }
      sub.error(new Error(`unexpected operation ${opName}`));
    }),
);

function renderMove() {
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return renderHook(
    () => ({
      list: useEnvironments("prj-1"),
      setter: useSetEnvironmentDatabases(),
    }),
    { wrapper },
  );
}

type Envs = { name: string; databaseIds: string[] }[];
const membership = (envs: Envs) =>
  Object.fromEntries(envs.map((e) => [e.name, e.databaseIds]));
const writes = () =>
  server.log.filter((e) => e === "→SetEnvironmentDatabases").length;
const moved = { stage: [DB], preview: [] };

beforeEach(() => {
  server = {
    membership: { stage: [], preview: [DB] },
    log: [],
    hold: false,
    releaseAfterWrite: false,
    failListsAfterWrite: 0,
    written: false,
    held: [],
  };
  vi.mocked(toast.success).mockReset();
  vi.mocked(toast.error).mockReset();
  vi.mocked(toast.warning).mockReset();
});

describe("moving a database lists it in its new environment once the move settles (w4/325)", () => {
  it("even when an older list read answers after the move", async () => {
    // The page's first Environments read is still in flight at Move.
    server.hold = true;
    server.releaseAfterWrite = true;
    const { result } = renderMove();
    await waitFor(() => expect(server.held).toHaveLength(1));

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setter.setDatabases("evm-stage", "stage", [DB]);
    });

    expect(ok).toBe(true);
    await waitFor(() =>
      expect(membership(result.current.list.environments)).toEqual(moved),
    );
    // …and still so once every older read has answered.
    await act(() => new Promise((resolve) => setTimeout(resolve, 60)));
    expect(server.held).toHaveLength(0);
    expect(membership(result.current.list.environments)).toEqual(moved);
    expect(writes()).toBe(1);
    // The list that published it was requested after the write answered.
    const ack = server.log.indexOf("←SetEnvironmentDatabases");
    expect(server.log.indexOf("→Environments", ack)).toBeGreaterThan(ack);
    expect(toast.success).toHaveBeenCalledWith(
      'Databases for "stage" updated.',
    );
  });

  it("on a settled page moves it with exactly one write", async () => {
    const { result } = renderMove();
    await waitFor(() =>
      expect(membership(result.current.list.environments)).toEqual({
        stage: [],
        preview: [DB],
      }),
    );

    await act(async () => {
      await result.current.setter.setDatabases("evm-stage", "stage", [DB]);
    });
    await waitFor(() =>
      expect(membership(result.current.list.environments)).toEqual(moved),
    );
    expect(writes()).toBe(1);
  });

  it("a failed refresh is not a failed move, and its retry only re-reads", async () => {
    const { result } = renderMove();
    await waitFor(() =>
      expect(result.current.list.environments).toHaveLength(2),
    );
    server.failListsAfterWrite = 99;

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setter.setDatabases("evm-stage", "stage", [DB]);
    });
    expect(ok).toBe(true);
    expect(toast.error).not.toHaveBeenCalled();
    expect(toast.warning).toHaveBeenCalledTimes(1);

    server.failListsAfterWrite = 0;
    const retry = vi.mocked(toast.warning).mock.calls[0]?.[1] as unknown as {
      action: { onClick: () => void };
    };
    await act(async () => {
      retry.action.onClick();
    });
    await waitFor(() =>
      expect(membership(result.current.list.environments)).toEqual(moved),
    );
    expect(writes()).toBe(1);
  });

  it("a refused move is an error with no refresh", async () => {
    server.refusal = 'forbidden: "dpg-x" does not belong to workspace "tea-1"';
    const { result } = renderMove();
    await waitFor(() =>
      expect(result.current.list.environments).toHaveLength(2),
    );
    const reads = server.log.filter((e) => e === "→Environments").length;

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setter.setDatabases("evm-stage", "stage", [
        "dpg-x",
      ]);
    });
    expect(ok).toBe(false);
    expect(toast.error).toHaveBeenCalledTimes(1);
    expect(server.log.filter((e) => e === "→Environments")).toHaveLength(reads);
  });
});
