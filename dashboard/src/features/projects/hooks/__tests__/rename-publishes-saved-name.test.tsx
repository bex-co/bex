import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider, useQuery } from "@apollo/client/react";
import { Observable } from "rxjs";
import {
  HeadContent,
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { toast } from "sonner";
import { apolloCacheConfig } from "@/common/apollo/cache";
import {
  formatDashboardTitle,
  type DashboardHead,
} from "@/common/lib/document-head";
import { ProjectsDocument } from "@/graphql/definitions";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { Route as ProjectRoute } from "@/routes/project.$projectId";
import { ProjectPage } from "@/routes/project.$projectId.index";
import {
  ProjectSettingsPage,
  Route as SettingsRoute,
} from "@/routes/project.$projectId.settings";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

// The Overview's resource table is not under test: rename touches no rows.
const noRows = { loading: false, error: undefined, refetch: vi.fn() };
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({ ...noRows, services: [] }),
}));
vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({ pending: null, run: vi.fn() }),
}));
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => ({ ...noRows, databases: [] }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => ({ ...noRows, keyValues: [] }),
}));
vi.mock("@/features/environments/components/environments-panel", () => ({
  EnvironmentsPanel: () => <div data-testid="environments-panel" />,
}));

/**
 * w4/m186: a Projects list read that started before RenameProject finished
 * after it, writing the old name over the mutation's normalized Project; the
 * project page's retained loader then re-ran cache-first and published the
 * old heading/title. Real Apollo cache, real Router loader/head (the actual
 * project route's loader and head), real pages; only the network is faked, as
 * a server whose responses carry the state AT REQUEST TIME and whose Projects
 * responses can be held — the pre-save snapshot arriving late.
 */
interface FakeServer {
  name: string;
  log: string[];
  holdProjects: boolean;
  /** Release held Projects responses right after the rename is acknowledged. */
  releaseAfterRename: boolean;
  /** Another actor renames to this right after our mutation commits. */
  concurrentName?: string;
  renameRefusal?: string;
  failProjectReadsAfterRename: number;
  renamed: boolean;
  /** Projects reads for this owner fail at the transport. */
  failingOwner?: string;
  /** Projects responses waiting for release(). */
  held: Array<() => void>;
}

let server: FakeServer;

function newServer(): FakeServer {
  return {
    name: "r3",
    log: [],
    holdProjects: false,
    releaseAfterRename: false,
    failProjectReadsAfterRename: 0,
    renamed: false,
    held: [],
  };
}

function release() {
  server.holdProjects = false;
  server.held.splice(0).forEach((send) => send());
}

const project = () => ({
  __typename: "Project",
  id: "prj-1",
  name: server.name,
  ownerId: "tea-1",
  createdAt: "2026-10-08T00:00:00Z",
  serviceIds: ["srv-1"],
  databaseIds: [],
  keyValueIds: [],
});

const link = new ApolloLink(
  (op) =>
    new Observable((sub) => {
      const opName = op.operationName ?? "";
      const v = op.variables as {
        id?: string;
        name?: string;
        ownerId?: string;
      };
      server.log.push(`→${opName}`);
      const send = (result: Record<string, unknown>) =>
        setTimeout(() => {
          server.log.push(`←${opName}`);
          sub.next(result);
          sub.complete();
        }, 0);
      const fail = (message: string) =>
        setTimeout(() => {
          server.log.push(`✗${opName}`);
          sub.error(new Error(message));
        }, 0);

      if (opName === "RenameProject") {
        if (server.renameRefusal) {
          send({
            errors: [
              {
                message: server.renameRefusal,
                extensions: { code: "BAD_USER_INPUT" },
              },
            ],
          });
          return;
        }
        server.name = v.name!;
        server.renamed = true;
        send({ data: { renameProject: project() } });
        if (server.concurrentName) server.name = server.concurrentName;
        if (server.releaseAfterRename) setTimeout(release, 0);
        return;
      }
      if (opName === "Project") {
        if (server.renamed && server.failProjectReadsAfterRename > 0) {
          server.failProjectReadsAfterRename -= 1;
          fail("503 Service Unavailable");
          return;
        }
        send({ data: { project: project() } });
        return;
      }
      if (opName === "Projects") {
        if (v.ownerId === server.failingOwner) {
          fail("503 Service Unavailable");
          return;
        }
        // The snapshot is the state when the read STARTED.
        const result = { data: { projects: [project()] } };
        if (server.holdProjects) server.held.push(() => send(result));
        else send(result);
        return;
      }
      sub.error(new Error(`unexpected operation ${opName}`));
    }),
);

/** The sidebar/breadcrumb stand-in: a real useProjects watcher. */
function SidebarLabel() {
  const { projects } = useProjects({ poll: false });
  return (
    <nav aria-label="sidebar">
      {projects.find((p) => p.id === "prj-1")?.name}
    </nav>
  );
}

/** A second, failing Projects watcher (another workspace's list). */
function FailingWatcher() {
  useQuery(ProjectsDocument, { variables: { ownerId: "tea-2" } });
  return null;
}

// The real file routes' loader/head, re-mounted under a test root (their own
// root carries auth/i18n beforeLoads this test does not need).
type RealLoader = (ctx: unknown) => Promise<unknown>;
type RealHead = (ctx: unknown) => DashboardHead;

function renderProject(
  page: "overview" | "settings",
  { sidebar = true, failingWatcher = false } = {},
) {
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  const rootRoute = createRootRoute({
    // The app's root beforeLoad awaits the (memoized) Kratos session on every
    // load, invalidate included, so a loader never reads the cache in the
    // same tick that triggered it.
    beforeLoad: () => new Promise<void>((resolve) => setTimeout(resolve, 5)),
    component: () => (
      <>
        <HeadContent />
        {sidebar ? <SidebarLabel /> : null}
        {failingWatcher ? <FailingWatcher /> : null}
        <Outlet />
      </>
    ),
  });
  const projectRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId",
    loader: ProjectRoute.options.loader as RealLoader,
    head: ProjectRoute.options.head as RealHead,
  });
  const childRoute =
    page === "overview"
      ? createRoute({
          getParentRoute: () => projectRoute,
          path: "/",
          component: ProjectPage,
        })
      : createRoute({
          getParentRoute: () => projectRoute,
          path: "settings",
          loader: SettingsRoute.options.loader as RealLoader,
          head: SettingsRoute.options.head as RealHead,
          component: ProjectSettingsPage,
        });
  const router = createRouter({
    routeTree: rootRoute.addChildren([projectRoute.addChildren([childRoute])]),
    history: createMemoryHistory({
      initialEntries: [
        page === "overview" ? "/project/prj-1" : "/project/prj-1/settings",
      ],
    }),
    context: { client, session: null } as never,
  });
  render(
    <ApolloProvider client={client}>
      <RouterProvider router={router} />
    </ApolloProvider>,
  );
}

const overviewTitle = (name: string) => formatDashboardTitle(name);
const settingsTitle = (name: string) =>
  formatDashboardTitle(`${name} / Settings`);

/** Rename through the Overview's pencil → dialog → Save. */
async function renameOnOverview(to: string) {
  const user = userEvent.setup();
  await screen.findByRole("heading", { name: "r3" });
  await user.click(screen.getByRole("button", { name: "Edit" }));
  const dialog = await screen.findByRole("dialog");
  const input = within(dialog).getByRole("textbox", { name: "Project name" });
  await user.clear(input);
  await user.type(input, to);
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  return dialog;
}

/** Rename through the Settings card's Edit → ✓. */
async function renameOnSettings(to: string) {
  const user = userEvent.setup();
  await screen.findByDisplayValue("r3");
  await user.click(screen.getByRole("button", { name: "Edit" }));
  const input = screen.getByRole("textbox", { name: "Project Name" });
  await user.clear(input);
  await user.type(input, to);
  await user.click(screen.getByRole("button", { name: "Save" }));
}

const count = (entry: string) =>
  server.log.filter((line) => line === entry).length;

/** Let every timer-driven response and loader run settle. */
const settle = () => new Promise((resolve) => setTimeout(resolve, 60));

beforeEach(() => {
  server = newServer();
  vi.mocked(toast.success).mockReset();
  vi.mocked(toast.error).mockReset();
  vi.mocked(toast.warning).mockReset();
  document.head.querySelectorAll("title").forEach((el) => el.remove());
});

afterEach(release);

describe("project rename publishes the saved name (w4/m186)", () => {
  it("Overview: a pre-save Projects read finishing after the mutation cannot stay published", async () => {
    // The sidebar's first list read starts before the rename and is held
    // until just after the server acknowledges it — finding.md's 918 → 1520 →
    // 1533 ms ordering.
    server.holdProjects = true;
    server.releaseAfterRename = true;
    renderProject("overview");

    const dialog = await renameOnOverview("r4");

    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));
    await settle();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("r4");
    expect(document.title).toBe(overviewTitle("r4"));
    expect(
      screen.getByRole("navigation", { name: "sidebar" }),
    ).toHaveTextContent("r4");
    expect(dialog).not.toBeInTheDocument();
    expect(count("→RenameProject")).toBe(1);
    // The late pre-save list answered with the OLD name, and the
    // authoritative by-id read only started after it had been written.
    const lastList = server.log.lastIndexOf("←Projects");
    const finalRead = server.log.lastIndexOf("→Project");
    expect(lastList).toBeGreaterThan(server.log.indexOf("←RenameProject"));
    expect(finalRead).toBeGreaterThan(lastList);
    expect(toast.error).not.toHaveBeenCalled();
    expect(toast.warning).not.toHaveBeenCalled();
  });

  it("Settings: the same late read cannot stay published", async () => {
    server.holdProjects = true;
    server.releaseAfterRename = true;
    renderProject("settings");

    await renameOnSettings("r4");

    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));
    await settle();
    expect(screen.getByDisplayValue("r4")).toBeDisabled(); // left edit mode
    expect(document.title).toBe(settingsTitle("r4"));
    expect(
      screen.getByRole("navigation", { name: "sidebar" }),
    ).toHaveTextContent("r4");
    expect(count("→RenameProject")).toBe(1);
  });

  it("keeps the dialog busy until the late read drains, then publishes", async () => {
    server.holdProjects = true; // released by hand, below
    renderProject("overview");

    const dialog = await renameOnOverview("r4");

    await waitFor(() => expect(count("←RenameProject")).toBe(1));
    await settle();
    // Saved, but the old list read is still pending: no completion yet.
    expect(within(dialog).getByRole("button", { name: "Save" })).toBeDisabled();
    expect(toast.success).not.toHaveBeenCalled();
    expect(count("→Project")).toBe(1); // only the entry load so far

    release();

    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("r4");
    expect(document.title).toBe(overviewTitle("r4"));
    expect(count("→RenameProject")).toBe(1);
  });

  it("an already-settled page renames normally (control)", async () => {
    renderProject("overview");
    await waitFor(() =>
      expect(
        screen.getByRole("navigation", { name: "sidebar" }),
      ).toHaveTextContent("r3"),
    );

    await renameOnOverview("r4");

    await waitFor(() => expect(document.title).toBe(overviewTitle("r4")));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("r4");
    expect(
      screen.getByRole("navigation", { name: "sidebar" }),
    ).toHaveTextContent("r4");
    expect(count("→RenameProject")).toBe(1);
  });

  it("publishes with no active Projects watcher", async () => {
    renderProject("overview", { sidebar: false });

    await renameOnOverview("r4");

    await waitFor(() => expect(document.title).toBe(overviewTitle("r4")));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("r4");
    expect(count("→Projects")).toBe(0);
    expect(count("→RenameProject")).toBe(1);
  });

  it("waits for every list read even when another rejects first", async () => {
    server.holdProjects = true;
    server.releaseAfterRename = true;
    server.failingOwner = "tea-2";
    renderProject("overview", { failingWatcher: true });

    await renameOnOverview("r4");

    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));
    await settle();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("r4");
    expect(document.title).toBe(overviewTitle("r4"));
    // A list failure is not a refresh failure: the by-id read is what
    // publishes, and it ran after the held list wrote.
    expect(toast.warning).not.toHaveBeenCalled();
    expect(server.log.lastIndexOf("→Project")).toBeGreaterThan(
      server.log.lastIndexOf("←Projects"),
    );
    expect(count("→RenameProject")).toBe(1);
  });

  it("a refused rename stays a refused rename", async () => {
    server.renameRefusal = "a project with that name already exists";
    renderProject("overview");

    const dialog = await renameOnOverview("r4");

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "A project with that name already exists",
      ),
    );
    await settle();
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Save" })).toBeEnabled();
    expect(
      screen.getByRole("heading", { level: 1, hidden: true }),
    ).toHaveTextContent("r3");
    expect(toast.success).not.toHaveBeenCalled();
    expect(toast.warning).not.toHaveBeenCalled();
    expect(count("→RenameProject")).toBe(1);
    expect(count("→Project")).toBe(1); // no post-write read
  });

  it("a failed refresh keeps the save, never re-sends it, and retries only the read", async () => {
    server.failProjectReadsAfterRename = 1;
    renderProject("overview");

    const dialog = await renameOnOverview("r4");

    await waitFor(() => expect(toast.warning).toHaveBeenCalledTimes(1));
    expect(vi.mocked(toast.warning).mock.calls[0][0]).toBe(
      'Project renamed to "r4", but this page couldn\'t refresh. Try again or reload to see the saved name.',
    );
    await waitFor(() => expect(dialog).not.toBeInTheDocument()); // busy released
    expect(toast.error).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();

    // The toast's retry re-reads; it does not rename again.
    const options = vi.mocked(toast.warning).mock.calls[0][1] as {
      action: { label: string; onClick: () => void };
    };
    expect(options.action.label).toBe("Try again");
    options.action.onClick();

    await waitFor(() => expect(document.title).toBe(overviewTitle("r4")));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("r4");
    expect(toast.success).toHaveBeenCalledTimes(1);
    expect(count("→RenameProject")).toBe(1);
  });

  it("publishes a newer name another actor saved meanwhile", async () => {
    server.concurrentName = "r5";
    renderProject("overview");

    await renameOnOverview("r4");

    await waitFor(() => expect(document.title).toBe(overviewTitle("r5")));
    await settle();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("r5");
    expect(
      screen.getByRole("navigation", { name: "sidebar" }),
    ).toHaveTextContent("r5");
    expect(toast.warning).not.toHaveBeenCalled();
    expect(count("→RenameProject")).toBe(1);
  });
});
