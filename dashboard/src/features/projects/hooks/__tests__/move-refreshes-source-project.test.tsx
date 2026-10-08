import { describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { Observable } from "rxjs";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ProjectDocument } from "@/graphql/definitions";
import { titleLoaderFetchPolicy } from "@/common/lib/document-head";
import {
  useMoveToProject,
  type UseMoveToProjectResult,
} from "@/features/projects/hooks/use-move-to-project";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

// The move picker's project list; the server state below is what matters.
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [
      {
        id: "prj-a",
        name: "A",
        ownerId: "tea-1",
        serviceIds: ["srv-1"],
        databaseIds: [],
        keyValueIds: [],
      },
      {
        id: "prj-b",
        name: "B",
        ownerId: "tea-1",
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      },
    ],
  }),
}));

// w4/m178: after a cross-project move the source project page kept the moved
// row until a reload. Real Apollo cache + real Router: the page's loader
// re-runs cache-first on invalidate (a retained match), and the move mutation
// returns only the TARGET Project, so the source entity in the cache is stale
// unless the hook re-reads it before invalidating. Only the network is faked,
// as a tiny server holding single-valued membership.
describe("Move to project → source project page", () => {
  it("drops the moved service from the source page without a reload", async () => {
    const members: Record<string, string[]> = {
      "prj-a": ["srv-1"],
      "prj-b": [],
    };
    const project = (id: string) => ({
      __typename: "Project",
      id,
      name: id === "prj-a" ? "A" : "B",
      ownerId: "tea-1",
      createdAt: "2026-10-07T00:00:00Z",
      serviceIds: members[id],
      databaseIds: [],
      keyValueIds: [],
    });
    const link = new ApolloLink(
      (op) =>
        new Observable((sub) => {
          const v = op.variables as { id: string; serviceIds?: string[] };
          let data: Record<string, unknown>;
          if (op.operationName === "SetProjectServices") {
            for (const k of Object.keys(members)) {
              members[k] = members[k].filter((s) => !v.serviceIds!.includes(s));
            }
            members[v.id] = v.serviceIds!;
            data = { setProjectServices: project(v.id) };
          } else if (op.operationName === "Project") {
            data = { project: project(v.id) };
          } else {
            data = { projects: Object.keys(members).map(project) };
          }
          sub.next({ data });
          sub.complete();
        }),
    );
    const client = new ApolloClient({
      link,
      cache: new InMemoryCache(apolloCacheConfig),
    });

    let mover: UseMoveToProjectResult | undefined;
    function ProjectPage() {
      mover = useMoveToProject("service");
      // Typed by hand: inferring through `route` here would be circular.
      const ids: string[] = route.useLoaderData() as string[];
      return (
        <ul aria-label="members">
          {ids.map((id) => (
            <li key={id}>{id}</li>
          ))}
        </ul>
      );
    }
    const root = createRootRoute();
    const route = createRoute({
      getParentRoute: () => root,
      path: "/move-probe/$projectId",
      loader: async ({ params, cause }) => {
        const { data } = await client.query({
          query: ProjectDocument,
          variables: { id: params.projectId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
        });
        return data?.project?.serviceIds ?? [];
      },
      component: ProjectPage,
    });
    const router = createRouter({
      routeTree: root.addChildren([route]),
      history: createMemoryHistory({ initialEntries: ["/move-probe/prj-a"] }),
    });
    render(
      <ApolloProvider client={client}>
        <RouterProvider router={router} />
      </ApolloProvider>,
    );
    await waitFor(() => expect(screen.getByText("srv-1")).toBeInTheDocument());

    await act(async () => {
      expect(await mover!.moveTo("srv-1", "web", "prj-b")).toBe(true);
    });

    await waitFor(() =>
      expect(screen.queryByText("srv-1")).not.toBeInTheDocument(),
    );
    expect(members).toEqual({ "prj-a": [], "prj-b": ["srv-1"] });
  });
});
