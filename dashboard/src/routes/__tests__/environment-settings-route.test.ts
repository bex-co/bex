import { describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter } from "@tanstack/react-router";
import { routeTree } from "@/routeTree.gen";
import { Route as SettingsRoute } from "../project.$projectId.environment.$environmentId.settings";
import { Route as PageRoute } from "../project.$projectId.environment.$environmentId.$page";

// w1/m683: Render's `/project/<id>/environment/<envId>/settings`, its other
// environment pages landing on the project, and the environment's own URL
// still the project page (w1/m682).
describe("environment routes", () => {
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never },
  });
  const leaf = (path: string) => router.matchRoutes(path).at(-1)?.routeId;

  it("keeps the environment's URL on the project page and serves its settings", () => {
    expect(leaf("/project/prj-1/environment/evm-1")).toBe(
      "/project/$projectId/_overview/environment/$environmentId",
    );
    expect(leaf("/project/prj-1/environment/evm-1/settings")).toBe(
      "/project/$projectId/environment/$environmentId/settings",
    );
    expect(leaf("/project/prj-1/environment/evm-1/deploys")).toBe(
      "/project/$projectId/environment/$environmentId/$page",
    );
  });

  it("sends any other environment page to the project", () => {
    const beforeLoad = PageRoute.options.beforeLoad as (ctx: {
      params: Record<string, string>;
    }) => never;
    let thrown: unknown;
    try {
      beforeLoad({
        params: { projectId: "prj-1", environmentId: "evm-1", page: "x" },
      });
    } catch (error) {
      thrown = error;
    }
    expect(thrown).toMatchObject({
      options: { to: "/project/$projectId", params: { projectId: "prj-1" } },
    });
  });

  type Loader = (ctx: unknown) => Promise<unknown>;
  const loader = SettingsRoute.options.loader as unknown as Loader;
  const environments = [
    { id: "evm-staging", projectId: "prj-1", name: "Staging" },
    { id: "evm-prod", projectId: "prj-1", name: "Production" },
  ];
  const load = (environmentId: string) =>
    loader({
      context: {
        client: {
          query: vi.fn(async () => ({ data: { environments } })),
        },
      },
      params: { projectId: "prj-1", environmentId },
      cause: "enter",
      parentMatchPromise: Promise.resolve({
        loaderData: { state: "ready", resource: { name: "storefront" } },
      }),
    });

  it("loads the environment for Render's title", async () => {
    const data = await load("evm-staging");
    expect(data).toMatchObject({
      projectName: "storefront",
      environment: { state: "ready", resource: { name: "Staging" } },
    });
    const head = SettingsRoute.options.head as unknown as (ctx: {
      loaderData: unknown;
      match: object;
    }) => { meta: Array<{ title?: string }> };
    expect(head({ loaderData: data, match: {} }).meta[0]?.title).toMatch(
      /^storefront \/ Staging \/ Settings ・ /,
    );
  });

  it("lands an id the project has no environment for on the project", async () => {
    await expect(load("evm-gone")).rejects.toMatchObject({
      options: { to: "/project/$projectId", params: { projectId: "prj-1" } },
    });
  });
});
