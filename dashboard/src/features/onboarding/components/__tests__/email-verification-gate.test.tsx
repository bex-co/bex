import { render, screen, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRouteWithContext,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import type { Session } from "@ory/client-fetch";
import { describe, expect, it } from "vitest";
import type { RouterContext } from "@/common/types/router-context";
import { EmailVerificationGate } from "../email-verification-gate";

/** A Kratos whoami session whose trait email is `email` and whose
 * verifiable_addresses are `addresses` (omit to drop the list entirely). */
function session(
  email: string | undefined,
  addresses?: Array<{ value: string; verified: boolean }>,
): Session {
  return {
    id: "ses-1",
    identity: {
      id: "id-1",
      schema_id: "default",
      schema_url: "",
      traits: email === undefined ? {} : { email },
      ...(addresses
        ? {
            verifiable_addresses: addresses.map((a) => ({
              ...a,
              via: "email",
              status: a.verified ? "completed" : "sent",
            })),
          }
        : {}),
    },
  } as unknown as Session;
}

function renderGate(
  sessionValue: Session | null,
  initialPath = "/",
  opts: { wallNeverLoads?: boolean } = {},
) {
  // The gate reads the browser location (not the memory router's) for `next`,
  // the same way the payment gate and the login bounce do.
  window.history.replaceState({}, "", initialPath);
  const rootRoute = createRootRouteWithContext<RouterContext>()();
  const appRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <EmailVerificationGate>
        <p>App content</p>
      </EmailVerificationGate>
    ),
  });
  const wallRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/auth/verification",
    validateSearch: (search: Record<string, unknown>) => ({
      next: typeof search.next === "string" ? search.next : undefined,
    }),
    // Holds the navigation pending so the gate's in-flight state is visible.
    loader: opts.wallNeverLoads ? () => new Promise<void>(() => {}) : undefined,
    component: () => <p>Verification wall</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([appRoute, wallRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: sessionValue },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

describe("EmailVerificationGate (ADR075 D8 revision, w2/m168)", () => {
  it("sends an unverified session to the wall with the current href as next", async () => {
    const { router } = renderGate(
      session("dev@example.com", [
        { value: "dev@example.com", verified: false },
      ]),
      "/services/new?type=web",
    );

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/auth/verification"),
    );
    expect(router.state.location.search).toEqual({
      next: "/services/new?type=web",
    });
    expect(await screen.findByText("Verification wall")).toBeInTheDocument();
    expect(screen.queryByText("App content")).not.toBeInTheDocument();
  });

  it("paints the verification route's own skeleton, never app content, while the redirect is in flight", async () => {
    const { container } = renderGate(
      session("dev@example.com", [
        { value: "dev@example.com", verified: false },
      ]),
      "/",
      { wallNeverLoads: true },
    );
    await waitFor(() =>
      expect(
        container.querySelector('[data-route-skeleton="auth-verification"]'),
      ).not.toBeNull(),
    );
    expect(screen.queryByText("App content")).not.toBeInTheDocument();
  });

  it("walls a session whose only verified address is not its trait email", async () => {
    const { router } = renderGate(
      session("new@example.com", [
        { value: "old@example.com", verified: true },
        { value: "new@example.com", verified: false },
      ]),
    );
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/auth/verification"),
    );
  });

  it("walls a session with an empty verifiable_addresses list", async () => {
    const { router } = renderGate(session("dev@example.com", []));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/auth/verification"),
    );
  });

  it("renders the app for a verified session (case-insensitive match)", async () => {
    const { router } = renderGate(
      session("Dev@Example.com", [
        { value: "dev@example.com", verified: true },
      ]),
    );
    expect(await screen.findByText("App content")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  // Fail-open: anything indeterminate renders the page; bex-api's
  // EMAIL_VERIFICATION_REQUIRED 403 is the backstop.
  it("fails open with no session (requireAuth owns that case)", async () => {
    renderGate(null);
    expect(await screen.findByText("App content")).toBeInTheDocument();
  });

  it("fails open when the session carries no verifiable_addresses list", async () => {
    renderGate(session("dev@example.com"));
    expect(await screen.findByText("App content")).toBeInTheDocument();
  });

  it("fails open when the identity has no trait email", async () => {
    renderGate(
      session(undefined, [{ value: "dev@example.com", verified: false }]),
    );
    expect(await screen.findByText("App content")).toBeInTheDocument();
  });
});
