import { describe, it, expect } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { KRATOS_PUBLIC_URL } from "@/common/lib/ory/config";
import { useKratosLinkNavigation } from "../use-kratos-link-navigation";

// The hook resolves against the configured Kratos base, so build the hrefs
// Ory would render from that same value rather than a hardcoded host.
const KRATOS = KRATOS_PUBLIC_URL;

type AnchorSpec = {
  label: string;
  href: string;
  target?: string;
  download?: boolean;
  testid?: string;
  nested?: boolean;
};

/**
 * Mounts the hook the way AuthPageShell does — on a column wrapping anchors it
 * does not own — and asserts against the router's own location, i.e. that a
 * CLIENT-SIDE navigation happened rather than a document load.
 */
function renderAnchors(anchors: AnchorSpec[]) {
  const rootRoute = createRootRoute();
  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/auth/login",
    validateSearch: (s: Record<string, unknown>) => s,
    component: function LoginHarness() {
      const onClick = useKratosLinkNavigation();
      return (
        <div onClick={onClick}>
          {anchors.map((a) => (
            <a
              key={a.label}
              href={a.href}
              target={a.target}
              data-testid={a.testid}
              {...(a.download ? { download: "" } : {})}
            >
              {a.nested ? <span>{a.label}</span> : a.label}
            </a>
          ))}
        </div>
      );
    },
  });
  const stub = (path: string) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      validateSearch: (s: Record<string, unknown>) => s,
      component: () => <div>{path}</div>,
    });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      loginRoute,
      stub("/auth/sign-up"),
      stub("/auth/forgot-password"),
      stub("/auth/verification"),
    ]),
    history: createMemoryHistory({ initialEntries: ["/auth/login"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

/** The router mounts asynchronously; wait for the anchor before clicking it. */
async function anchorByText(label: string) {
  return screen.findByText(label);
}

const SIGN_UP = `${KRATOS}/self-service/registration/browser?return_to=https%3A%2F%2Fdashboard.bex.co%2F`;

describe("useKratosLinkNavigation", () => {
  it("navigates client-side instead of leaving for Kratos", async () => {
    const user = userEvent.setup();
    const router = renderAnchors([{ label: "Sign up", href: SIGN_UP }]);
    await user.click(await anchorByText("Sign up"));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/auth/sign-up"),
    );
  });

  it("routes the recovery link to the forgot-password page", async () => {
    const user = userEvent.setup();
    const router = renderAnchors([
      { label: "Recover Account", href: `${KRATOS}/self-service/recovery/browser` },
    ]);
    await user.click(await anchorByText("Recover Account"));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/auth/forgot-password"),
    );
  });

  it("carries login_challenge across the hop", async () => {
    const user = userEvent.setup();
    const router = renderAnchors([
      {
        label: "Sign up",
        href: `${KRATOS}/self-service/registration/browser?login_challenge=hydra-abc`,
      },
    ]);
    await user.click(await anchorByText("Sign up"));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/auth/sign-up"),
    );
    expect(router.state.location.search).toMatchObject({
      login_challenge: "hydra-abc",
    });
  });

  it("still navigates when the click lands on a nested element", async () => {
    const user = userEvent.setup();
    const router = renderAnchors([
      { label: "Sign up", href: SIGN_UP, nested: true },
    ]);
    await user.click(await anchorByText("Sign up"));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/auth/sign-up"),
    );
  });

  // Everything below must behave exactly as it did before the interceptor —
  // the fallback is always the browser's own handling, never a broken click.
  it.each([
    ["Meta", "{Meta>}"],
    ["Control", "{Control>}"],
    ["Shift", "{Shift>}"],
    ["Alt", "{Alt>}"],
  ])("leaves a %s-click for the browser (open in new tab)", async (_n, key) => {
    const user = userEvent.setup();
    const router = renderAnchors([{ label: "Sign up", href: SIGN_UP }]);
    await user.keyboard(key);
    await user.click(await anchorByText("Sign up"));
    expect(router.state.location.pathname).toBe("/auth/login");
  });

  it("leaves a middle-click alone", async () => {
    const user = userEvent.setup();
    const router = renderAnchors([{ label: "Sign up", href: SIGN_UP }]);
    await user.pointer({
      target: await anchorByText("Sign up"),
      keys: "[MouseMiddle]",
    });
    expect(router.state.location.pathname).toBe("/auth/login");
  });

  it.each([
    ["target=_blank", { label: "Sign up", href: SIGN_UP, target: "_blank" }],
    ["download", { label: "Sign up", href: SIGN_UP, download: true }],
    [
      "a flow restart",
      {
        label: "Adjust",
        href: `${KRATOS}/self-service/login/browser`,
        testid: "ory/screen/login/action/restart",
      },
    ],
    [
      "an unrelated link",
      { label: "Docs", href: "https://docs.bex.co/guide" },
    ],
    [
      "a non-self-service Kratos link",
      { label: "Logout", href: `${KRATOS}/self-service/logout?token=t` },
    ],
  ])("does not intercept %s", async (_name, anchor) => {
    const user = userEvent.setup();
    const router = renderAnchors([anchor as AnchorSpec]);
    await user.click(await anchorByText((anchor as AnchorSpec).label));
    expect(router.state.location.pathname).toBe("/auth/login");
  });
});
