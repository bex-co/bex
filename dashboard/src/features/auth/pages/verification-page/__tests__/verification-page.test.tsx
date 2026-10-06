import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  FlowType,
  VerificationFlowState,
  type VerificationFlow,
} from "@ory/client-fetch";

// The page is a thin client of useOryFlow + Ory Elements' <Verification>. Mock
// both so the test drives the loading vs. ready states directly without a live
// Kratos or a router-mounted flow effect.
const oryFlow = vi.hoisted(() => ({ value: null as VerificationFlow | null }));
vi.mock("@/common/hooks/use-ory-flow", () => ({
  useOryFlow: (...args: unknown[]) => {
    calls.push(args);
    return oryFlow.value;
  },
}));
const calls: unknown[][] = [];

// Expose onSuccess (and the flow Elements was handed) so a test can complete
// the code step the way Elements does and inspect the pre-fill.
const elements = vi.hoisted(() => ({
  onSuccess: null as null | ((event: unknown) => void),
  flow: null as VerificationFlow | null,
}));
vi.mock("@ory/elements-react/theme", () => ({
  Verification: ({
    onSuccess,
    flow,
  }: {
    onSuccess: (event: unknown) => void;
    flow: VerificationFlow;
  }) => {
    elements.onSuccess = onSuccess;
    elements.flow = flow;
    return <div data-testid="ory-verification" />;
  },
}));

const sessionCache = vi.hoisted(() => ({ invalidate: vi.fn() }));
vi.mock("@/common/server-fn/session", () => ({
  invalidateSessionCache: sessionCache.invalidate,
}));

vi.mock("@/common/lib/ory/config", () => ({
  useOryConfig: () => ({}),
  oryHideCardLogo: {},
  KRATOS_PUBLIC_URL: "http://localhost",
}));

// The page reads its `flow`/`next` search params and navigates on success;
// pin both (no router mount).
const routerMock = vi.hoisted(() => ({
  search: { flow: undefined, next: undefined } as {
    flow: string | undefined;
    next: string | undefined;
  },
  navigate: vi.fn(),
  session: null as unknown,
}));
vi.mock("@tanstack/react-router", async (orig) => ({
  ...(await orig<typeof import("@tanstack/react-router")>()),
  useSearch: () => routerMock.search,
  useNavigate: () => routerMock.navigate,
  useRouteContext: () => ({ session: routerMock.session }),
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => (
    <a href={to}>{children}</a>
  ),
}));

import VerificationPage from "@/features/auth/pages/verification-page";

/** A signed-in session whose trait email is (un)verified. */
const signedIn = (verified: boolean) => ({
  id: "ses-1",
  identity: {
    id: "id-1",
    traits: { email: "dev@example.com" },
    verifiable_addresses: [{ value: "dev@example.com", verified }],
  },
});

/** A fresh choose_method flow with an empty email input, as Kratos mints it. */
const chooseMethodFlow = () =>
  ({
    id: "flow-1",
    state: VerificationFlowState.ChooseMethod,
    ui: {
      action: "",
      method: "POST",
      nodes: [
        {
          type: "input",
          group: "code",
          attributes: { node_type: "input", name: "email", type: "email" },
          messages: [],
          meta: {},
        },
      ],
    },
  }) as unknown as VerificationFlow;

const prefilled = () =>
  (elements.flow?.ui.nodes[0]?.attributes as { value?: unknown } | undefined)
    ?.value;

beforeEach(() => {
  oryFlow.value = null;
  calls.length = 0;
  elements.onSuccess = null;
  elements.flow = null;
  routerMock.search = { flow: undefined, next: undefined };
  routerMock.navigate.mockReset();
  routerMock.session = null;
  sessionCache.invalidate.mockReset();
});

describe("VerificationPage", () => {
  it("requests a Kratos verification flow", () => {
    render(<VerificationPage />);
    // First arg is the flow kind — this is what wires the page to the right
    // Kratos self-service flow; a regression to e.g. "recovery" would break it.
    expect(calls[0]?.[0]).toBe("verification");
  });

  it("shows a loading skeleton until the flow resolves", () => {
    oryFlow.value = null;
    render(<VerificationPage />);
    expect(screen.queryByTestId("ory-verification")).not.toBeInTheDocument();
    // The hero copy renders regardless of flow state.
    expect(screen.getByText("Verify your email")).toBeInTheDocument();
  });

  it("renders the Ory verification form once the flow is ready", () => {
    oryFlow.value = { id: "flow-1" } as VerificationFlow;
    render(<VerificationPage />);
    expect(screen.getByTestId("ory-verification")).toBeInTheDocument();
  });

  // ADR075 D7 (revised 2026-08-29): a verified sign-up continues through the
  // payment wall, which carries the guarded deep link onward.
  it("continues into the product through the payment wall, deep link intact", () => {
    oryFlow.value = { id: "flow-1" } as VerificationFlow;
    routerMock.search = { flow: undefined, next: "/services/new?type=web" };
    render(<VerificationPage />);
    elements.onSuccess?.({
      flowType: FlowType.Verification,
      flow: { state: VerificationFlowState.PassedChallenge },
    });
    expect(routerMock.navigate).toHaveBeenCalledWith({
      to: "/",
      href: "/setup/payment?next=%2Fservices%2Fnew%3Ftype%3Dweb",
    });
  });

  it("does not navigate on the intermediate 'code sent' submit", () => {
    oryFlow.value = { id: "flow-1" } as VerificationFlow;
    render(<VerificationPage />);
    elements.onSuccess?.({
      flowType: FlowType.Verification,
      flow: { state: VerificationFlowState.SentEmail },
    });
    expect(routerMock.navigate).not.toHaveBeenCalled();
  });

  it("drops an off-origin deep link before it reaches the wall", () => {
    oryFlow.value = { id: "flow-1" } as VerificationFlow;
    routerMock.search = { flow: undefined, next: "https://evil.example/" };
    render(<VerificationPage />);
    elements.onSuccess?.({
      flowType: FlowType.Verification,
      flow: { state: VerificationFlowState.PassedChallenge },
    });
    expect(routerMock.navigate).toHaveBeenCalledWith({
      to: "/",
      href: "/setup/payment",
    });
  });
});

// ADR075 D8 revision (2026-10-06, w2/m168): the page doubles as the wall a
// signed-in unverified session is sent to, with `?next=` and no flow id.
describe("VerificationPage as the verification wall (w2/m168)", () => {
  it("pre-fills the session's own email into a fresh flow", () => {
    routerMock.session = signedIn(false);
    oryFlow.value = chooseMethodFlow();
    render(<VerificationPage />);
    expect(prefilled()).toBe("dev@example.com");
  });

  it("does not pre-fill anything for a session-less visitor", () => {
    oryFlow.value = chooseMethodFlow();
    render(<VerificationPage />);
    expect(prefilled()).toBeUndefined();
  });

  it("says why a signed-in unverified session is here and offers sign-out", () => {
    routerMock.session = signedIn(false);
    render(<VerificationPage />);
    expect(
      screen.getByText("Verify your email address to keep using bex"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Sign out" })).toHaveAttribute(
      "href",
      "/auth/logout",
    );
  });

  it("keeps the ordinary copy and no sign-out for a session-less visitor", () => {
    render(<VerificationPage />);
    expect(
      screen.getByText(
        "Enter the email address associated with your account to continue",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Sign out" })).toBeNull();
  });

  it("drops the memoized whoami before continuing, so the gate sees the verified session", () => {
    routerMock.session = signedIn(false);
    routerMock.search = { flow: undefined, next: "/services" };
    oryFlow.value = chooseMethodFlow();
    render(<VerificationPage />);
    elements.onSuccess?.({
      flowType: FlowType.Verification,
      flow: { state: VerificationFlowState.PassedChallenge },
    });
    expect(sessionCache.invalidate).toHaveBeenCalledTimes(1);
    expect(sessionCache.invalidate.mock.invocationCallOrder[0]).toBeLessThan(
      routerMock.navigate.mock.invocationCallOrder[0],
    );
    expect(routerMock.navigate).toHaveBeenCalledWith({
      to: "/",
      href: "/setup/payment?next=%2Fservices",
    });
  });
});
