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
  resourceUnauthenticated,
  useNotFoundRedirect,
} from "../use-not-found-redirect";
import { ServerError } from "@apollo/client/errors";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";

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
  const notFoundErr = codedGraphQLError("NOT_FOUND");
  const outage = new Error("Failed to fetch");

  it("a dead id is not-found even though the backend reports it as an error", () => {
    expect(resourceNotFound(null, false, notFoundErr)).toBe(true);
    expect(resourceFailed(null, false, notFoundErr)).toBe(false);
  });

  // The code decides (w5/m130): a feature's own *_NOT_FOUND counts, and the
  // wording alone, without a code, no longer does.
  // bex-api answers a missing resource 200 with a coded error, so a transport
  // 404 is an ingress or proxy failure: retry, never redirect away.
  it("treats a transport 404 as a failure, not a missing resource", () => {
    const proxy404 = new ServerError("status 404", {
      response: new Response("no", { status: 404 }),
      bodyText: "no",
    });
    expect(resourceNotFound(null, false, proxy404)).toBe(false);
    expect(resourceFailed(null, false, proxy404)).toBe(true);
  });

  it("decides not-found by code, not by 'not found' in the wording", () => {
    expect(
      resourceNotFound(
        null,
        false,
        codedGraphQLError("AGENT_SESSION_NOT_FOUND"),
      ),
    ).toBe(true);
    const worded = uncodedGraphQLError("deploy not found");
    expect(resourceNotFound(null, false, worded)).toBe(false);
    expect(resourceFailed(null, false, worded)).toBe(true);
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

// w3/m80 t002: an expired session (bex-api 401) must never wear the network
// error's "check the API and try again" card — it gets its own predicate so the
// detail page can offer Sign in instead, while genuine 5xx/transport failures
// keep the retry card.
describe("resourceUnauthenticated (w3/m80)", () => {
  const unauthorized = new ServerError("status 401", {
    response: new Response("no", { status: 401 }),
    bodyText: "no",
  });
  const outage = new ServerError("status 502", {
    response: new Response("no", { status: 502 }),
    bodyText: "no",
  });
  const notFoundErr = codedGraphQLError("NOT_FOUND");

  it("claims a 401 and takes it away from resourceFailed", () => {
    expect(resourceUnauthenticated(null, false, unauthorized)).toBe(true);
    expect(resourceFailed(null, false, unauthorized)).toBe(false);
  });

  it("leaves a real outage with the network error card", () => {
    expect(resourceUnauthenticated(null, false, outage)).toBe(false);
    expect(resourceFailed(null, false, outage)).toBe(true);
  });

  it("does not claim a not-found or a still-loading query", () => {
    expect(resourceUnauthenticated(null, false, notFoundErr)).toBe(false);
    expect(resourceUnauthenticated(null, true, unauthorized)).toBe(false);
  });

  it("is inert once the resource resolved", () => {
    expect(resourceUnauthenticated({ id: "srv-1" }, false, unauthorized)).toBe(
      false,
    );
  });
});
