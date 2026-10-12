import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { toast } from "sonner";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { useCreateEnvironment } from "../use-create-environment";
import { useEnvironments } from "../use-environments";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

/**
 * w4/231: an Environments read that started before CreateEnvironment answered
 * after it, without the new environment. The create's named refetch deduped
 * onto that in-flight read, so the dialog closed over a list missing what it
 * had just created until the next 30s poll. Real Apollo client and cache; only
 * the network is faked, as a server whose list responses carry the state AT
 * REQUEST TIME and can be held — the pre-save snapshot arriving late.
 */
interface FakeServer {
  environments: string[];
  log: string[];
  hold: boolean;
  /** Release held list responses right after the create is acknowledged. */
  releaseAfterCreate: boolean;
  /** List reads after the create fail at the transport. */
  failListsAfterCreate: number;
  createRefusal?: string;
  created: boolean;
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

      if (opName === "CreateEnvironment") {
        if (server.createRefusal) {
          send({
            errors: [
              {
                message: server.createRefusal,
                extensions: { code: "BAD_USER_INPUT" },
              },
            ],
          });
          return;
        }
        const name = (op.variables as { name: string }).name;
        server.environments = [...server.environments, name];
        server.created = true;
        send({ data: { createEnvironment: environment(name) } });
        // Late enough that, unless the hook waits for it, the stale list
        // lands after the hook's own fresh read.
        if (server.releaseAfterCreate) setTimeout(release, 20);
        return;
      }
      if (opName === "Environments") {
        if (server.created && server.failListsAfterCreate > 0) {
          server.failListsAfterCreate -= 1;
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

function renderCreate() {
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
      creator: useCreateEnvironment("prj-1"),
    }),
    { wrapper },
  );
}

const names = (envs: { name: string }[]) => envs.map((e) => e.name);
const creates = () =>
  server.log.filter((entry) => entry === "→CreateEnvironment").length;

beforeEach(() => {
  server = {
    environments: [],
    log: [],
    hold: false,
    releaseAfterCreate: false,
    failListsAfterCreate: 0,
    created: false,
    held: [],
  };
  vi.mocked(toast.success).mockReset();
  vi.mocked(toast.error).mockReset();
  vi.mocked(toast.warning).mockReset();
});

describe("creating an environment lists it once the create settles (w4/231)", () => {
  it.each([
    ["an initially empty project", [] as string[]],
    ["a project with environments", ["stage", "preview"]],
  ])(
    "on %s, even when an older list read answers after the create",
    async (_case, existing) => {
      server.environments = existing;
      // The page's first Environments read is still in flight at submit.
      server.hold = true;
      server.releaseAfterCreate = true;
      const { result } = renderCreate();
      await waitFor(() => expect(server.held).toHaveLength(1));

      let id: string | null | undefined;
      await act(async () => {
        id = await result.current.creator.create("dev");
      });

      expect(id).toBe("evm-dev");
      // On completion — waitFor's 1s bound is far inside the 30s poll, and
      // nothing reloads.
      await waitFor(() =>
        expect(names(result.current.list.environments)).toEqual([
          ...existing,
          "dev",
        ]),
      );
      // …and still listed once every older read has answered.
      await act(() => new Promise((resolve) => setTimeout(resolve, 60)));
      expect(server.held).toHaveLength(0);
      expect(names(result.current.list.environments)).toEqual([
        ...existing,
        "dev",
      ]);
      expect(result.current.creator.busy).toBe(false);
      expect(creates()).toBe(1);
      // The list that published it was requested after the create answered.
      const ack = server.log.indexOf("←CreateEnvironment");
      expect(server.log.indexOf("→Environments", ack)).toBeGreaterThan(ack);
      expect(toast.success).toHaveBeenCalledWith('Environment "dev" created.');
    },
  );

  it("on a settled page lists it with exactly one create", async () => {
    server.environments = ["stage"];
    const { result } = renderCreate();

    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage"]),
    );

    await act(async () => {
      await result.current.creator.create("settled");
    });
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual([
        "stage",
        "settled",
      ]),
    );
    expect(creates()).toBe(1);
    // With nothing in flight, the refresh is one fresh read — no refetch.
    const ack = server.log.indexOf("←CreateEnvironment");
    expect(
      server.log.slice(ack).filter((e) => e === "→Environments"),
    ).toHaveLength(1);
  });

  it("a failed refresh is not a failed create, and its retry only re-reads", async () => {
    server.environments = ["stage"];
    const { result } = renderCreate();
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage"]),
    );
    server.failListsAfterCreate = 99;

    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.creator.create("dev");
    });
    expect(id).toBe("evm-dev");
    expect(toast.error).not.toHaveBeenCalled();
    expect(toast.warning).toHaveBeenCalledTimes(1);
    // Existing rows stay; the refresh is named incomplete.
    expect(names(result.current.list.environments)).toEqual(["stage"]);

    server.failListsAfterCreate = 0;
    const retry = vi.mocked(toast.warning).mock.calls[0]?.[1] as unknown as {
      action: { onClick: () => void };
    };
    await act(async () => {
      retry.action.onClick();
    });
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage", "dev"]),
    );
    expect(creates()).toBe(1);
    expect(toast.success).toHaveBeenCalledWith('Environment "dev" created.');
  });

  it("a refused create is a creation error with no refresh", async () => {
    server.environments = ["stage"];
    server.createRefusal = 'environment "stage" already exists';
    const { result } = renderCreate();
    await waitFor(() =>
      expect(names(result.current.list.environments)).toEqual(["stage"]),
    );
    const reads = server.log.filter((e) => e === "→Environments").length;

    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.creator.create("stage");
    });
    expect(id).toBeNull();
    expect(toast.error).toHaveBeenCalledTimes(1);
    expect(toast.success).not.toHaveBeenCalled();
    expect(server.log.filter((e) => e === "→Environments")).toHaveLength(reads);
  });
});
