import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { RequiresSshKey } from "@/features/ssh-keys/components/requires-ssh-key";
import type { HasSSHKeyState } from "@/features/ssh-keys/hooks/use-has-ssh-key";

const hookState: HasSSHKeyState = {
  hasKey: false,
  loading: false,
  error: false,
};
vi.mock("@/features/ssh-keys/hooks/use-has-ssh-key", () => ({
  useHasSSHKey: () => hookState,
}));

const client = new ApolloClient({
  cache: new InMemoryCache(),
  link: ApolloLink.empty(),
});

function withClient(ui: React.ReactNode) {
  return <ApolloProvider client={client}>{ui}</ApolloProvider>;
}

const gate = (
  <RequiresSshKey fallback={<span>ADD KEY CTA</span>}>
    <span>REAL AFFORDANCE</span>
  </RequiresSshKey>
);

beforeEach(() => {
  hookState.hasKey = false;
  hookState.loading = false;
  hookState.error = false;
});

describe("RequiresSshKey", () => {
  it("renders the real affordance when the caller has a key", () => {
    hookState.hasKey = true;
    render(withClient(gate));
    expect(screen.getByText("REAL AFFORDANCE")).toBeInTheDocument();
    expect(screen.queryByText("ADD KEY CTA")).not.toBeInTheDocument();
  });

  it("swaps in the CTA only when the caller is confirmed to have no key", () => {
    hookState.hasKey = false;
    render(withClient(gate));
    expect(screen.getByText("ADD KEY CTA")).toBeInTheDocument();
    expect(screen.queryByText("REAL AFFORDANCE")).not.toBeInTheDocument();
  });

  it("fails open while the key query is loading (never hides a working feature)", () => {
    hookState.loading = true;
    render(withClient(gate));
    expect(screen.getByText("REAL AFFORDANCE")).toBeInTheDocument();
    expect(screen.queryByText("ADD KEY CTA")).not.toBeInTheDocument();
  });

  it("fails open when the key query errors", () => {
    hookState.error = true;
    render(withClient(gate));
    expect(screen.getByText("REAL AFFORDANCE")).toBeInTheDocument();
    expect(screen.queryByText("ADD KEY CTA")).not.toBeInTheDocument();
  });

  it("fails open when there is no Apollo client at all (isolated render)", () => {
    // No ApolloProvider: the gate can't ask, so it must show the real affordance,
    // not a spurious CTA. hookState says no-key, but it is never consulted.
    hookState.hasKey = false;
    render(gate);
    expect(screen.getByText("REAL AFFORDANCE")).toBeInTheDocument();
    expect(screen.queryByText("ADD KEY CTA")).not.toBeInTheDocument();
  });
});
