import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type RouteComponent,
} from "@tanstack/react-router";
import { Route as GenerateRoute } from "@/routes/blueprint.generate";
import { ROUTE_SKELETON_MANIFEST } from "@/common/lib/route-skeleton-manifest";
import { BlueprintGenerateFrame } from "@/common/components/route-skeletons";
import { downloadTextFile } from "@/common/lib/download-file";
import { GenerateBlueprintDocument } from "@/graphql/definitions";
import type {
  BlueprintExport,
  BlueprintExportResource,
} from "@/features/blueprints/lib/blueprint-export";

// w3/m681: Render's Create Blueprint page (`ExportBlueprint-*.js`), opened by
// Generate Blueprint with the resources in the navigation's state.

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));
vi.mock("@/features/workspaces/context/hooks", async () => {
  const { mockWorkspaceContext } = await import("@/test/mocks/workspace");
  return mockWorkspaceContext("tea-test");
});
vi.mock("@/common/lib/download-file", () => ({ downloadTextFile: vi.fn() }));

const query = vi.hoisted(() => ({
  result: {} as { data?: unknown; error?: Error },
  calls: [] as { document: unknown; options: Record<string, unknown> }[],
}));
vi.mock("@apollo/client/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@apollo/client/react")>()),
  useQuery: (document: unknown, options: Record<string, unknown>) => {
    query.calls.push({ document, options });
    return { loading: !query.result.data, ...query.result };
  },
}));

const MANIFEST = "services:\n  - type: web\n    name: web\n";

const web: BlueprintExportResource = {
  kind: "service",
  id: "srv-web",
  name: "web",
};
const worker: BlueprintExportResource = {
  kind: "service",
  id: "srv-worker",
  name: "worker",
};
const pg: BlueprintExportResource = {
  kind: "database",
  id: "dpg-1",
  name: "orders-db",
};
const kv: BlueprintExportResource = {
  kind: "keyvalue",
  id: "red-1",
  name: "cache",
};
const group: BlueprintExportResource = {
  kind: "envgroup",
  id: "evg-1",
  name: "shared",
};

function handed(
  resources: BlueprintExportResource[],
  extra: Partial<BlueprintExport> = {},
): BlueprintExport {
  return { resources, source: "action bar", ...extra };
}

/** The page at `/blueprint/generate`, opened from `/` with `state`. */
async function open(state?: BlueprintExport) {
  const root = createRootRoute();
  const home = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: () => <p>home</p>,
  });
  const page = createRoute({
    getParentRoute: () => root,
    path: "/blueprint/generate",
    component: GenerateRoute.options.component as RouteComponent,
    loader: GenerateRoute.options.loader as never,
  });
  const create = createRoute({
    getParentRoute: () => root,
    path: "/blueprint/new",
    component: () => <p>create</p>,
  });
  const router = createRouter({
    // The file Route's options are typed for the generated tree, not this one.
    routeTree: root.addChildren([home, page as never, create]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  await screen.findByText("home");
  await act(() =>
    router.navigate({
      to: "/blueprint/generate",
      ...(state ? { state: { blueprintExport: state } } : {}),
    } as never),
  );
  return router;
}

const lastVariables = () => query.calls.at(-1)?.options.variables;

beforeEach(() => {
  query.result = {
    data: {
      generateBlueprint: { manifest: MANIFEST, filename: "render.yaml" },
    },
  };
  query.calls = [];
  vi.mocked(downloadTextFile).mockReset();
});

describe("the Create Blueprint page (w3/m681)", () => {
  it.each([
    [[web], "Includes 1 service: web"],
    [[web, pg], "Includes 2 services: web and orders-db"],
    [[web, pg, kv], "Includes 3 services: web, orders-db, and cache"],
  ])(
    "names %j in Render's sentence, each name in bold",
    async (resources, sentence) => {
      await open(handed(resources));
      const line = await screen.findByText(/^Includes/);
      expect(line).toHaveTextContent(sentence);
      expect(
        [...line.querySelectorAll(".font-semibold")].map((b) => b.textContent),
      ).toStrictEqual(resources.map((r) => r.name));
    },
  );

  it("asks generateBlueprint for each kind's ids, naming services first", async () => {
    await open(handed([group, pg, web, kv, worker]));
    expect(await screen.findByText(/^Includes 5 services/)).toHaveTextContent(
      "Includes 5 services: web, worker, orders-db, cache, and shared",
    );
    expect(query.calls.at(-1)?.document).toBe(GenerateBlueprintDocument);
    expect(lastVariables()).toStrictEqual({
      ownerId: "tea-test",
      serviceIds: ["srv-web", "srv-worker"],
      postgresIds: ["dpg-1"],
      keyValueIds: ["red-1"],
      envGroupIds: ["evg-1"],
    });
    expect(query.calls.at(-1)?.options.fetchPolicy).toBe("no-cache");
  });

  it("titles the page for the environment it came from, else for your services", async () => {
    const names = { projectName: "storefront", environmentName: "Production" };
    await open(handed([web], { source: "project environment menu", ...names }));
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Create a new Blueprint from storefront / Production",
      }),
    ).toBeVisible();

    await open(handed([web], { source: "table row menu", ...names }));
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Create a new Blueprint from your services",
      }),
    ).toBeVisible();
  });

  it("names the same in the document title, from the loader", () => {
    const head = GenerateRoute.options.head as (ctx: unknown) => {
      meta: { title?: string }[];
    };
    const title = (loaderData: unknown) =>
      head({ match: {}, loaderData }).meta[0]?.title;
    expect(
      title({
        handed: handed([web], {
          source: "project environment menu",
          projectName: "storefront",
          environmentName: "Production",
        }),
      }),
    ).toBe(
      "Create a new Blueprint from storefront / Production ・ bex Dashboard",
    );
    expect(title({ handed: null })).toBe(
      "Create a new Blueprint from your services ・ bex Dashboard",
    );
  });

  it("goes to / when nothing was handed over, as Render's does", async () => {
    const router = await open();
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
    expect(await screen.findByText("home")).toBeVisible();
    expect(query.calls).toStrictEqual([]);
  });

  it("downloads render.yaml and copies it, reading Copied! for two seconds", async () => {
    const user = userEvent.setup();
    await open(handed([web]));
    const code = await screen.findByText(/name: web/);
    expect(code.textContent).toBe(MANIFEST);

    await user.click(
      screen.getByRole("button", { name: "Download render.yaml" }),
    );
    expect(downloadTextFile).toHaveBeenCalledWith(
      "render.yaml",
      MANIFEST,
      "application/yaml",
    );

    await user.click(screen.getByRole("button", { name: "Copy" }));
    expect(await navigator.clipboard.readText()).toBe(MANIFEST);
    expect(screen.getByRole("button", { name: "Copied!" })).toBeVisible();
    await waitFor(
      () => expect(screen.getByRole("button", { name: "Copy" })).toBeVisible(),
      { timeout: 3000 },
    );
  });

  it("shows Render's loading box until the file arrives, with every action disabled", async () => {
    query.result = {};
    await open(handed([web]));
    expect(await screen.findByText("Generating render.yaml...")).toBeVisible();
    expect(screen.queryByText(/^Includes/)).toBeNull();
    for (const name of [
      "Download render.yaml",
      "Copy",
      "Connect a new Blueprint",
    ]) {
      expect(screen.getByRole("button", { name })).toBeDisabled();
    }
  });

  it("shows the server's refusal in place of the file", async () => {
    query.result = {
      error: new Error("Forbidden: generate needs can_view_sensitive"),
    };
    await open(handed([web]));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Couldn't generate the Blueprint",
    );
    expect(
      screen.getByRole("button", { name: "Download render.yaml" }),
    ).toBeDisabled();
  });

  it("walks Render's three steps and opens the create page in a new tab", async () => {
    await open(handed([web]));
    await screen.findByText(/^Includes/);
    const steps = within(document.querySelector<HTMLElement>("main ol")!)
      .getAllByRole("heading", { level: 2 })
      .map((h) => h.textContent);
    expect(steps).toStrictEqual([
      "Download generated render.yaml",
      "Add render.yaml to Git",
      "Connect your new Blueprint",
    ]);
    expect(screen.getByText(/^git add render.yaml/).textContent).toBe(
      'git add render.yaml\ngit commit -m "Add render.yaml"\ngit push',
    );
    const connect = screen.getByRole("link", {
      name: "Connect a new Blueprint",
    });
    expect(connect).toHaveAttribute("href", "/blueprint/new");
    expect(connect).toHaveAttribute("target", "_blank");
    expect(
      screen.getByRole("link", { name: "Learn about Blueprints" }),
    ).toHaveAttribute("href", "https://bex.co/docs/platform/blueprints");
    // Render's second aside link, Preview Environments, is a non-goal.
    expect(screen.queryByText(/Preview Environments/)).toBeNull();
  });

  it("draws its pending frame with the page's regions and steps", async () => {
    const { container, unmount } = render(
      <BlueprintGenerateFrame title="Create a new Blueprint from your services" />,
    );
    const frame = container.querySelector(
      '[data-route-skeleton="blueprint-generate"]',
    );
    expect(frame).not.toBeNull();
    const regions = [
      ...frame!.querySelectorAll<HTMLElement>("[data-skeleton-region]"),
    ].map((el) => el.dataset.skeletonRegion);
    for (const region of ROUTE_SKELETON_MANIFEST["/blueprint/generate"]
      .regions) {
      expect(regions).toContain(region);
    }
    const pendingSteps = [...frame!.querySelectorAll("ol > li h2")].map(
      (h) => h.textContent,
    );
    expect(frame!.querySelector("ol")?.textContent).toContain(
      "Generating render.yaml...",
    );
    unmount();

    query.result = {};
    await open(handed([web]));
    await screen.findByText("Generating render.yaml...");
    const readySteps = [...document.querySelectorAll("main ol > li h2")].map(
      (h) => h.textContent,
    );
    expect(readySteps).toStrictEqual(pendingSteps);
  });
});
