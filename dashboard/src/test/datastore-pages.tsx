import { render } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type RouteComponent,
} from "@tanstack/react-router";
import { Route as DatabaseLayoutRoute } from "@/routes/databases.$databaseId";
import { Route as KeyValueLayoutRoute } from "@/routes/keyvalue.$keyValueId";

interface PageRoute {
  options: {
    component?: unknown;
    validateSearch?: unknown;
    beforeLoad?: unknown;
  };
}

/**
 * Mounts a datastore's layout route with some of its page routes under it,
 * from their file Routes' own options (w3/m641, w3/m649), at `href`. The
 * pages are keyed by their path under the layout (`"/"`, `"/logs"`, …), so
 * each reconstructed route keeps its file Route's id and its
 * `Route.useSearch()` reads the right match. The layout's loader and its
 * parent's auth guard stay out; the tests stub the data hooks instead.
 */
async function renderDatastorePages(
  layoutPath: string,
  layoutRoute: PageRoute,
  pages: Record<string, PageRoute>,
  href: string,
) {
  const root = createRootRoute();
  const layout = createRoute({
    getParentRoute: () => root,
    path: layoutPath,
    component: layoutRoute.options.component as RouteComponent,
  });
  const children = Object.entries(pages).map(([path, page]) =>
    createRoute({
      getParentRoute: () => layout,
      path,
      component: page.options.component as RouteComponent,
      validateSearch: page.options.validateSearch as never,
      beforeLoad: page.options.beforeLoad as never,
    }),
  );
  const router = createRouter({
    // The file Routes' options are typed for the generated tree, not this one.
    routeTree: root.addChildren([
      layout.addChildren(children as never) as never,
    ]),
    history: createMemoryHistory({ initialEntries: [href] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  render(<RouterProvider router={router} />);
  return router;
}

/** A Postgres database's layout and `pages` (w3/m641). */
export function renderDatabasePages(
  pages: Record<string, PageRoute>,
  href: string,
) {
  return renderDatastorePages(
    "/databases/$databaseId",
    DatabaseLayoutRoute,
    pages,
    href,
  );
}

/** A Key Value instance's layout and `pages` (w3/m649). */
export function renderKeyValuePages(
  pages: Record<string, PageRoute>,
  href: string,
) {
  return renderDatastorePages(
    "/keyvalue/$keyValueId",
    KeyValueLayoutRoute,
    pages,
    href,
  );
}

/**
 * A datastore layout's pending frame (its route's `pendingComponent`) at
 * `href`, with no page under it, as the router paints it while the title
 * loader runs (w3/m675).
 */
export function renderDatastorePendingFrame(
  kind: "database" | "keyValue",
  href: string,
) {
  const [path, route] =
    kind === "database"
      ? ["/databases/$databaseId", DatabaseLayoutRoute]
      : ["/keyvalue/$keyValueId", KeyValueLayoutRoute];
  return renderDatastorePages(
    path,
    { options: { component: route.options.pendingComponent } },
    {},
    href,
  );
}

/**
 * What sets a datastore header's geometry, since jsdom has no layout: its
 * row's classes, how many items its title group holds, and each action's
 * container, label and box classes. A pending frame's reserved box counts as
 * its ready control's box (w3/m675).
 */
export function datastoreHeaderGeometry(header: Element) {
  const [title, actions] = [...header.children];
  const box = (element: Element) =>
    element.matches("[data-slot=button]")
      ? element
      : (element.querySelector("[data-slot=button], .invisible") ?? element);
  const classes = (element: Element) =>
    element.className
      .split(" ")
      .filter((name) => name && name !== "invisible")
      .join(" ");
  return {
    row: classes(header),
    titleItems: title.children.length,
    actions: {
      container: classes(actions),
      boxes: [...actions.children].map((child) => ({
        label: box(child).textContent,
        classes: classes(box(child)),
      })),
    },
  };
}
