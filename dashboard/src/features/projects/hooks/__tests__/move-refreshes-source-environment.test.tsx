import { describe, expect, it, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRouter,
} from "@tanstack/react-router";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { EnvironmentsDocument } from "@/graphql/definitions";
import { EnvironmentsPanel } from "@/features/environments/components/environments-panel";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import {
  useMoveToProject,
  type ProjectResourceKind,
  type UseMoveToProjectResult,
} from "@/features/projects/hooks/use-move-to-project";
import type { ServiceView } from "@/features/services/types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast }));

// Polling disabled: the row must leave because of the move's own refresh, not
// because a 30s timer happened to fire.
vi.mock("@/common/lib/polling", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/common/lib/polling")>()),
  RESOURCE_POLL_INTERVAL_MS: 0,
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({ groups: [], loading: false, error: undefined }),
}));
// The shared table pulls in every row action; render names only. The card's
// own rows/counts (the consumer under test) stay real.
vi.mock("@/features/projects/components/resource-table", () => ({
  resourceSelectionKey: (row: { kind: string; id: string }) =>
    `${row.kind}:${row.id}`,
  ResourceTable: ({ rows }: { rows: Array<{ id: string }> }) => (
    <ul aria-label="rows">
      {rows.map((row) => (
        <li key={row.id}>{row.id}</li>
      ))}
    </ul>
  ),
}));
vi.mock("@/features/environments/components/manage-resources-dialog", () => ({
  ManageResourcesDialog: () => null,
}));

type Kind = "service" | "database" | "keyvalue";
const FIELD = {
  service: "serviceIds",
  database: "databaseIds",
  keyvalue: "keyValueIds",
} as const;
const MUTATION = {
  SetProjectServices: "service",
  SetProjectDatabases: "database",
  SetProjectKeyValues: "keyvalue",
} as const;
type Members = Record<Kind, string[]>;

// The picker's project list mirrors the server's initial state.
const initialProjects = (): Record<string, Members> => ({
  "prj-a": {
    service: ["srv-1", "srv-2"],
    database: ["dpg-1"],
    keyvalue: ["red-1"],
  },
  "prj-b": { service: ["srv-9"], database: [], keyvalue: [] },
  "prj-c": { service: ["srv-c"], database: [], keyvalue: [] },
});
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: Object.entries(initialProjects()).map(([id, m]) => ({
      id,
      name: id.toUpperCase(),
      ownerId: "tea-1",
      serviceIds: m.service,
      databaseIds: m.database,
      keyValueIds: m.keyvalue,
    })),
  }),
}));

/**
 * A tiny server with single-valued membership: a resource belongs to at most
 * one project and, within it, at most one environment. Environments reads can
 * be held to model a pre-write poll still in flight when the move commits.
 */
function makeServer() {
  const projects = initialProjects();
  const envs: Record<string, { projectId: string } & Members> = {
    "evm-stage": {
      projectId: "prj-a",
      service: ["srv-1", "srv-2"],
      database: ["dpg-1"],
      keyvalue: ["red-1"],
    },
    "evm-c": {
      projectId: "prj-c",
      service: ["srv-c"],
      database: [],
      keyvalue: [],
    },
  };
  const log: Array<{ op: string; vars: Record<string, unknown> }> = [];
  const held: Array<() => void> = [];
  const state = { hold: false, failMutation: false, failEnvRead: false };

  const project = (id: string) => ({
    __typename: "Project",
    id,
    name: id.toUpperCase(),
    ownerId: "tea-1",
    createdAt: "2026-10-11T00:00:00Z",
    serviceIds: projects[id].service,
    databaseIds: projects[id].database,
    keyValueIds: projects[id].keyvalue,
  });
  const environment = (id: string) => ({
    __typename: "Environment",
    id,
    projectId: envs[id].projectId,
    name: id === "evm-stage" ? "Stage" : "Other",
    ownerId: "tea-1",
    createdAt: "2026-10-11T00:00:00Z",
    serviceIds: [...envs[id].service],
    databaseIds: [...envs[id].database],
    keyValueIds: [...envs[id].keyvalue],
    envGroupIds: [],
    protectedStatus: "unprotected",
    networkIsolationEnabled: false,
    ipAllowList: [],
    ipAllowListEntries: [],
    ipAllowListProxiedDomains: [],
  });

  function setMembers(kind: Kind, id: string, wanted: string[]) {
    for (const removed of projects[id][kind].filter(
      (r) => !wanted.includes(r),
    )) {
      for (const env of Object.values(envs)) {
        env[kind] = env[kind].filter((r) => r !== removed);
      }
    }
    for (const added of wanted) {
      for (const [pid, m] of Object.entries(projects)) {
        if (pid !== id) m[kind] = m[kind].filter((r) => r !== added);
      }
      for (const env of Object.values(envs)) {
        if (env.projectId !== id)
          env[kind] = env[kind].filter((r) => r !== added);
      }
    }
    projects[id][kind] = [...wanted];
  }

  const link = new ApolloLink(
    (op) =>
      new Observable((sub) => {
        const vars = op.variables as Record<string, unknown>;
        log.push({ op: op.operationName ?? "", vars });
        const kind = MUTATION[op.operationName as keyof typeof MUTATION];
        if (kind) {
          if (state.failMutation) {
            sub.error(new Error("membership refused"));
            return;
          }
          const field = FIELD[kind];
          setMembers(kind, vars.id as string, vars[field] as string[]);
          const root = `${op.operationName![0].toLowerCase()}${op.operationName!.slice(1)}`;
          sub.next({ data: { [root]: project(vars.id as string) } });
          sub.complete();
          return;
        }
        if (op.operationName === "Environments") {
          // Snapshot at request time: a held read answers with what the
          // server held when it started, as a real in-flight poll would.
          const data = {
            environments: Object.keys(envs)
              .filter((id) => envs[id].projectId === vars.projectId)
              .map(environment),
          };
          const reply = () => {
            if (state.failEnvRead) sub.error(new Error("refresh failed"));
            else {
              sub.next({ data });
              sub.complete();
            }
          };
          if (state.hold) held.push(reply);
          else reply();
          return;
        }
        if (op.operationName === "Project") {
          sub.next({ data: { project: project(vars.id as string) } });
        } else {
          sub.next({ data: { projects: Object.keys(projects).map(project) } });
        }
        sub.complete();
      }),
  );
  return { projects, envs, log, held, state, link };
}

const services = ["srv-1", "srv-2", "srv-9", "srv-c"].map(
  (id) => ({ id, name: id, createdAt: null }) as unknown as ServiceView,
);
const databases = [
  { id: "dpg-1", name: "dpg-1", createdAt: null },
] as unknown as DatabaseView[];
const keyValues = [
  { id: "red-1", name: "red-1", createdAt: null },
] as unknown as KeyValueView[];

function renderSourceStage(server: ReturnType<typeof makeServer>) {
  const client = new ApolloClient({
    link: server.link,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  const handles: {
    mover?: UseMoveToProjectResult;
    closeMenu?: () => void;
    kind: ProjectResourceKind;
  } = { kind: "service" };

  // The dropdown closes as soon as an item is chosen, unmounting the hook
  // before its refresh finishes.
  function Menu() {
    handles.mover = useMoveToProject(handles.kind);
    return null;
  }
  // An unrelated project's Environments watcher must not be refreshed.
  function OtherProjectEnvironments() {
    useEnvironments("prj-c");
    return null;
  }
  function Page() {
    const [menuOpen, setMenuOpen] = useState(true);
    handles.closeMenu = () => setMenuOpen(false);
    return (
      <>
        {menuOpen ? <Menu /> : null}
        <OtherProjectEnvironments />
        <EnvironmentsPanel
          projectId="prj-a"
          services={services}
          databases={databases}
          keyValues={keyValues}
          servicePending={null}
          onRunServiceAction={vi.fn()}
          onDatabaseDeleted={vi.fn()}
          onKeyValueDeleted={vi.fn()}
          resourceFilter={{
            environmentId: "evm-stage",
            query: "",
            kind: "all",
          }}
        />
      </>
    );
  }
  const router = createRouter({
    routeTree: createRootRoute({ component: Page }),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  render(
    <ApolloProvider client={client}>
      <RouterProvider router={router} />
    </ApolloProvider>,
  );
  return { client, router, handles };
}

const allCount = () =>
  screen.getByRole("button", { name: /^All \(\d+\)$/ }).textContent;

const CASES = [
  { kind: "service", id: "srv-1", count: "Services (1)" },
  { kind: "database", id: "dpg-1", count: "Databases (0)" },
  { kind: "keyvalue", id: "red-1", count: "Key Values (0)" },
] as const;

beforeEach(() => {
  toast.success.mockReset();
  toast.error.mockReset();
});

// w4/m178 (pass 155): the selected source Environment kept a moved resource's
// row and count until its next poll, because its rows come from the separate
// Environments list that no Project read updates. Real Apollo cache, real
// EnvironmentsPanel/EnvironmentCard membership and counts; only the network is
// faked. An older pre-write list read is held until after the write commits
// so it cannot win by ordering luck.
describe("Move to project → selected source Environment", () => {
  it.each(CASES)(
    "drops a moved $kind from the source Environment once its re-read settles",
    async ({ kind, id, count }) => {
      const server = makeServer();
      const { client, handles } = renderSourceStage(server);
      handles.kind = kind;
      await waitFor(() => expect(screen.getByText(id)).toBeInTheDocument());
      expect(allCount()).toBe("All (4)");

      // A pre-write poll is still in flight when the move commits.
      server.state.hold = true;
      void client
        .refetchQueries({ include: [EnvironmentsDocument] })
        .catch(() => undefined);
      await waitFor(() => expect(server.held.length).toBeGreaterThan(0));

      let ok = false;
      await act(async () => {
        const moved = handles.mover!.moveTo(id, id, "prj-b");
        handles.closeMenu!();
        ok = await moved;
      });
      expect(ok).toBe(true);
      // Pending: previous data is kept, not cleared to a skeleton.
      expect(screen.getByText(id)).toBeInTheDocument();

      // Newest first: the pre-write list answers last, so it would overwrite
      // a fresh read issued without settling it first.
      server.state.hold = false;
      await act(async () => {
        for (const reply of server.held.splice(0).reverse()) reply();
      });

      await waitFor(() =>
        expect(screen.queryByText(id)).not.toBeInTheDocument(),
      );
      expect(allCount()).toBe("All (3)");
      expect(screen.getByRole("button", { name: count })).toBeInTheDocument();
      // Unrelated members stay; placement is authoritative on both sides.
      expect(screen.getByText("srv-2")).toBeInTheDocument();
      expect(server.projects["prj-a"][kind]).not.toContain(id);
      expect(server.projects["prj-b"][kind]).toContain(id);
      expect(server.projects["prj-b"].service).toContain("srv-9");
      expect(server.envs["evm-stage"][kind]).not.toContain(id);
      // Exactly one membership write; only the source's lists were re-read.
      const writes = server.log.filter((e) => e.op.startsWith("SetProject"));
      expect(writes).toHaveLength(1);
      const write = server.log.indexOf(writes[0]);
      const after = server.log.slice(write + 1);
      expect(
        after
          .filter((e) => e.op === "Environments")
          .map((e) => e.vars.projectId),
      ).toEqual(expect.arrayContaining(["prj-a"]));
      expect(
        after.some(
          (e) => e.op === "Environments" && e.vars.projectId !== "prj-a",
        ),
      ).toBe(false);
      expect(after.filter((e) => e.op === "Project")).toEqual([
        { op: "Project", vars: { id: "prj-a" } },
      ]);
      expect(toast.error).not.toHaveBeenCalled();
    },
  );

  it.each(CASES)(
    "drops a removed $kind from the source Environment without a Project read",
    async ({ kind, id, count }) => {
      const server = makeServer();
      const { handles } = renderSourceStage(server);
      handles.kind = kind;
      await waitFor(() => expect(screen.getByText(id)).toBeInTheDocument());

      await act(async () => {
        const removed = handles.mover!.removeFromProject(id, id);
        handles.closeMenu!();
        expect(await removed).toBe(true);
      });

      await waitFor(() =>
        expect(screen.queryByText(id)).not.toBeInTheDocument(),
      );
      expect(allCount()).toBe("All (3)");
      expect(screen.getByRole("button", { name: count })).toBeInTheDocument();
      expect(screen.getByText("srv-2")).toBeInTheDocument();
      // Ungrouped: in no project and no environment.
      for (const m of Object.values(server.projects))
        expect(m[kind]).not.toContain(id);
      expect(server.envs["evm-stage"][kind]).not.toContain(id);
      expect(
        server.log.filter((e) => e.op.startsWith("SetProject")),
      ).toHaveLength(1);
      expect(server.log.some((e) => e.op === "Project")).toBe(false);
    },
  );

  it("keeps the row and reports the failure when the move is refused", async () => {
    const server = makeServer();
    const { handles } = renderSourceStage(server);
    await waitFor(() => expect(screen.getByText("srv-1")).toBeInTheDocument());
    server.state.failMutation = true;
    const before = server.log.length;

    await act(async () => {
      expect(await handles.mover!.moveTo("srv-1", "web", "prj-b")).toBe(false);
    });

    expect(toast.error).toHaveBeenCalledTimes(1);
    expect(toast.success).not.toHaveBeenCalled();
    expect(server.log.slice(before).map((e) => e.op)).toEqual([
      "SetProjectServices",
    ]);
    expect(screen.getByText("srv-1")).toBeInTheDocument();
    expect(server.projects["prj-a"].service).toContain("srv-1");
  });

  it("treats a failed source re-read as freshness, never a failed move", async () => {
    const server = makeServer();
    const { handles, router } = renderSourceStage(server);
    await waitFor(() => expect(screen.getByText("srv-1")).toBeInTheDocument());
    const invalidate = vi.spyOn(router, "invalidate");
    server.state.failEnvRead = true;

    await act(async () => {
      expect(await handles.mover!.moveTo("srv-1", "web", "prj-b")).toBe(true);
    });
    await waitFor(() => expect(invalidate).toHaveBeenCalled());
    await waitFor(() =>
      expect(
        server.log.filter(
          (e) => e.op === "Environments" && e.vars.projectId === "prj-a",
        ).length,
      ).toBeGreaterThan(1),
    );

    expect(toast.success).toHaveBeenCalledTimes(1);
    expect(toast.error).not.toHaveBeenCalled();
    expect(
      server.log.filter((e) => e.op.startsWith("SetProject")),
    ).toHaveLength(1);
    expect(server.projects["prj-b"].service).toContain("srv-1");
  });
});
