import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { toast } from "sonner";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { useDeleteEnvironment } from "../use-delete-environment";
import { useEnvironments } from "../use-environments";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

/**
 * w4/290: an Environments read that started before DeleteEnvironment answered
 * after it, still listing the deleted environment. The delete's named refetch
 * deduped onto that in-flight read, so the dialog closed over a project that
 * kept the deleted environment selected and offered its id for a new service.
 * Real Apollo client and cache; only the network is faked, as a server whose
 * list responses carry the state AT REQUEST TIME and can be held.
 */
interface FakeServer {
  environments: string[];
  log: string[];
  hold: boolean;
  /** Release held list responses right after the delete is acknowledged. */
  releaseAfterDelete: boolean;
  /** List reads after the delete fail at the transport. */
  failListsAfterDelete: number;
  deleteRefusal?: string;
  deleted: boolean;
  held: Array<() => void>;
}

let server: FakeServer;

function environment(name: string) {
  return {
    __typename: "Environment",
    id: `evm-${name}`,
    projectId: "prj-1",
    name,
    ownerId: "tea-1",
    createdAt: "2026-10-10T08:46:15Z",
    serviceIds: [],
    databaseIds: [],
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

      if (opName === "DeleteEnvironment") {
        if (server.deleteRefusal) {
          send({
            errors: [
              {
                message: server.deleteRefusal,
                extensions: { code: "FORBIDDEN" },
              },
            ],
          });
          return;
        }
        const id = (op.variables as { id: string }).id;
        server.environments = server.environments.filter(
          (name) => `evm-${name}` !== id,
        );
        server.deleted = true;
        send({ data: { deleteEnvironment: true } });
        // Late enough that, unless the hook waits for it, the stale list
        // lands after the hook's own fresh read.
        if (server.releaseAfterDelete) setTimeout(release, 20);
        return;
      }
      if (opName === "Environments") {
        if (server.deleted && server.failListsAfterDelete > 0) {
          server.failListsAfterDelete -= 1;
          setTimeout(() => {
            server.log.push(`✗${opName}`);
            sub.error(new Error("ERR_CONNECTION_CLOSED"));
          }, 0);
          return;
        }
        // The snapshot is taken now, at request time.
        const result = {
          data: { environments: server.environments.map(environment) },
        };
        if (server.hold) server.held.push(() => send(result));
        else send(result);
        return;
      }
      sub.error(new Error(`unexpected operation ${opName}`));
    }),
);

function renderDelete() {
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
      deleter: useDeleteEnvironment("prj-1"),
    }),
    { wrapper },
  );
}

const names = (envs: { name: string }[]) => envs.map((e) => e.name);
const deletes = () =>
  server.log.filter((entry) => entry === "→DeleteEnvironment").length;

beforeEach(() => {
  server = {
    environments: [],
    log: [],
    hold: false,
    releaseAfterDelete: false,
    failListsAfterDelete: 0,
    deleted: false,
    held: [],
  };
  vi.mocked(toast.success).mockReset();
  vi.mocked(toast.error).mockReset();
  vi.mocked(toast.warning).mockReset();
});

describe("deleting an environment removes it from the list once the delete settles (w4/290)", () => {
  it.each([
    [
      "other environments remain",
      ["stage", "dev", "preview"],
      ["stage", "preview"],
    ],
    ["it was the project's only one", ["dev"], [] as string[]],
  ])(
    "when %s, even though an older list read answers after the delete",
    async (_case, existing, remaining) => {
      server.environments = existing;
      // A list read is still in flight when Delete is confirmed.
      server.hold = true;
      server.releaseAfterDelete = true;
      const { result } = renderDelete();
      await waitFor(() => expect(server.held).toHaveLength(1));

      let ok: boolean | undefined;
      await act(async () => {
        ok = await result.current.deleter.remove("evm-dev", "dev");
      });

      expect(ok).toBe(true);
      // On completion — waitFor's bound is far inside the 30s poll.
      await waitFor(() =>
        expect(names(result.current.list.environments)).toEqual(remaining),
      );
      expect(result.current.deleter.deleting).toBeNull();
      // …and it stays gone once every older read has answered.
      await act(() => new Promise((resolve) => setTimeout(resolve, 60)));
      expect(server.held).toHaveLength(0);
      expect(names(result.current.list.environments)).toEqual(remaining);
      expect(deletes()).toBe(1);
      // The list that published it was requested after the delete answered.
      const ack = server.log.indexOf("←DeleteEnvironment");
      expect(server.log.indexOf("→Environments", ack)).toBeGreaterThan(ack);
      expect(toast.success).toHaveBeenCalledWith('Environment "dev" deleted.');
    },
  );

  it("keeps the delete pending until the fresh list is published", async () => {
    server.environments = ["stage", "dev"];
    const { result } = renderDelete();
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage", "dev"]),
    );
    // Hold the post-delete read: the delete has answered, the list has not.
    server.hold = true;
    let done = false;
    act(() => {
      void result.current.deleter.remove("evm-dev", "dev").then(() => {
        done = true;
      });
    });
    await waitFor(() => expect(server.held).toHaveLength(1));
    expect(done).toBe(false);
    expect(result.current.deleter.deleting).toBe("evm-dev");
    // The rows on screen stay until the fresh list replaces them.
    expect(names(result.current.list.environments)).toEqual(["stage", "dev"]);

    await act(async () => {
      release();
      await new Promise((resolve) => setTimeout(resolve, 20));
    });
    expect(done).toBe(true);
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage"]),
    );
  });

  it("on a settled page removes it with exactly one delete and one fresh read", async () => {
    server.environments = ["stage", "dev"];
    const { result } = renderDelete();
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage", "dev"]),
    );
    await act(async () => {
      await result.current.deleter.remove("evm-dev", "dev");
    });
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage"]),
    );
    expect(deletes()).toBe(1);
    const ack = server.log.indexOf("←DeleteEnvironment");
    expect(
      server.log.slice(ack).filter((e) => e === "→Environments"),
    ).toHaveLength(1);
  });

  it("a failed refresh is not a failed delete, and its retry only re-reads", async () => {
    server.environments = ["stage", "dev"];
    const { result } = renderDelete();
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage", "dev"]),
    );
    server.failListsAfterDelete = 99;

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.deleter.remove("evm-dev", "dev");
    });
    expect(ok).toBe(true);
    expect(toast.error).not.toHaveBeenCalled();
    expect(toast.warning).toHaveBeenCalledTimes(1);
    expect(vi.mocked(toast.warning).mock.calls[0]?.[0]).toMatch(
      /deleted, but the list couldn't refresh/,
    );

    server.failListsAfterDelete = 0;
    const retry = vi.mocked(toast.warning).mock.calls[0]?.[1] as unknown as {
      action: { onClick: () => void };
    };
    await act(async () => {
      retry.action.onClick();
    });
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage"]),
    );
    expect(deletes()).toBe(1);
    expect(toast.success).toHaveBeenCalledWith('Environment "dev" deleted.');
  });

  it("a refused delete is a deletion error: the environment stays and nothing re-reads", async () => {
    server.environments = ["stage", "dev"];
    server.deleteRefusal = "forbidden";
    const { result } = renderDelete();
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage", "dev"]),
    );
    const reads = server.log.filter((e) => e === "→Environments").length;

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.deleter.remove("evm-dev", "dev");
    });
    expect(ok).toBe(false);
    expect(toast.error).toHaveBeenCalledTimes(1);
    expect(toast.success).not.toHaveBeenCalled();
    expect(names(result.current.list.environments)).toEqual(["stage", "dev"]);
    expect(server.log.filter((e) => e === "→Environments")).toHaveLength(reads);
  });
});
