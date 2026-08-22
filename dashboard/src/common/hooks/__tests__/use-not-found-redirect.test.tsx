import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import {
  resourceFailed,
  resourceNotFound,
  useNotFoundRedirect,
} from "../use-not-found-redirect";

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { error: (...args: unknown[]) => toastError(...args) },
}));

function Probe({ notFound }: { notFound: boolean }) {
  useNotFoundRedirect(notFound);
  return <div>detail page</div>;
}

function renderAt(notFound: boolean) {
  const rootRoute = createRootRoute();
  const home = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div>home page</div>,
  });
  const detail = createRoute({
    getParentRoute: () => rootRoute,
    path: "/things/$thingId",
    component: () => <Probe notFound={notFound} />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([home, detail]),
    history: createMemoryHistory({ initialEntries: ["/things/dead-id"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  toastError.mockReset();
});

describe("useNotFoundRedirect (w9/m55)", () => {
  it("replaces a dead resource URL with / and toasts why", async () => {
    const router = renderAt(true);

    expect(await screen.findByText("home page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
    // replace, not push: the dead URL must not stack a history entry.
    expect(router.history.length).toBe(1);
    expect(toastError).toHaveBeenCalledWith(
      "That resource doesn't exist or was deleted.",
      { id: "resource-not-found" },
    );
  });

  it("stays put while the resource is not provably absent", async () => {
    const router = renderAt(false);

    expect(await screen.findByText("detail page")).toBeInTheDocument();
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/things/dead-id"),
    );
    expect(toastError).not.toHaveBeenCalled();
  });
});

// w6/m44: the predicate every detail page feeds the hook above. bex-api answers
// a dead id with `null` AND an errors entry saying why, so "settled empty" and
// "no error" are different questions — reading only the second is what silently
// dropped w9/m55's redirect on /services, /databases, /keyvalue, /blueprints,
// and /webhook.
describe("resourceNotFound / resourceFailed (w6/m44)", () => {
  const resource = { id: "srv-1" };
  const notFoundErr = new Error("app not found");
  const outage = new Error("Failed to fetch");

  it("a dead id is not-found even though the backend reports it as an error", () => {
    expect(resourceNotFound(null, false, notFoundErr)).toBe(true);
    expect(resourceFailed(null, false, notFoundErr)).toBe(false);
  });

  it("a genuine failure is an error, never a redirect", () => {
    expect(resourceNotFound(null, false, outage)).toBe(false);
    expect(resourceFailed(null, false, outage)).toBe(true);
  });

  it("an empty settle with no error at all is still not-found", () => {
    expect(resourceNotFound(null, false, undefined)).toBe(true);
    expect(resourceFailed(null, false, undefined)).toBe(false);
  });

  it("neither fires while the query is still loading", () => {
    expect(resourceNotFound(null, true, undefined)).toBe(false);
    expect(resourceNotFound(null, true, notFoundErr)).toBe(false);
    expect(resourceFailed(null, true, outage)).toBe(false);
  });

  it("a resolved resource is neither, even alongside a stale error", () => {
    expect(resourceNotFound(resource, false, outage)).toBe(false);
    expect(resourceFailed(resource, false, outage)).toBe(false);
  });

  it("the two are exact complements over every settled-empty error", () => {
    for (const err of [undefined, notFoundErr, outage]) {
      expect(resourceNotFound(null, false, err)).toBe(
        !resourceFailed(null, false, err),
      );
    }
  });
});
