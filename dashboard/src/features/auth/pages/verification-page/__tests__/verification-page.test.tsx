import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { VerificationFlow } from "@ory/client-fetch";

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

vi.mock("@ory/elements-react/theme", () => ({
  Verification: () => <div data-testid="ory-verification" />,
}));

vi.mock("@/common/lib/ory/config", () => ({
  useOryConfig: () => ({}),
  oryHideCardLogo: {},
  KRATOS_PUBLIC_URL: "http://localhost",
}));

// The page reads its `flow` search param; return a fixed value (no router mount).
vi.mock("@tanstack/react-router", async (orig) => ({
  ...(await orig<typeof import("@tanstack/react-router")>()),
  useSearch: () => ({ flow: undefined }),
}));

import VerificationPage from "@/features/auth/pages/verification-page";

beforeEach(() => {
  oryFlow.value = null;
  calls.length = 0;
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
});
