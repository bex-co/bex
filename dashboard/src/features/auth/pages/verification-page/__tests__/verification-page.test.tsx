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

// Expose onSuccess so a test can complete the code step the way Elements does.
const elements = vi.hoisted(() => ({
  onSuccess: null as null | ((event: unknown) => void),
}));
vi.mock("@ory/elements-react/theme", () => ({
  Verification: ({ onSuccess }: { onSuccess: (event: unknown) => void }) => {
    elements.onSuccess = onSuccess;
    return <div data-testid="ory-verification" />;
  },
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
}));
vi.mock("@tanstack/react-router", async (orig) => ({
  ...(await orig<typeof import("@tanstack/react-router")>()),
  useSearch: () => routerMock.search,
  useNavigate: () => routerMock.navigate,
}));

import VerificationPage from "@/features/auth/pages/verification-page";

beforeEach(() => {
  oryFlow.value = null;
  calls.length = 0;
  elements.onSuccess = null;
  routerMock.search = { flow: undefined, next: undefined };
  routerMock.navigate.mockReset();
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
