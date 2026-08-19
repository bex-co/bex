import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ConnectGithubCard } from "@/features/git/components/connect-github-card";

const refetch = vi.fn();
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnections: () => ({
    connections: [],
    connected: false,
    loading: false,
    error: undefined,
    refetch,
  }),
}));

vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGit: () => ({ connect: vi.fn(), busy: false }),
}));

vi.mock("@/features/git/hooks/use-disconnect-git", () => ({
  useDisconnectGit: () => ({ disconnect: vi.fn(), busy: false }),
}));

beforeEach(() => {
  refetch.mockReset();
});

describe("ConnectGithubCard callback failures", () => {
  it("shows an expired callback as a visible retryable error", () => {
    render(<ConnectGithubCard callbackError="expired_state" />);

    expect(
      screen.getByText("GitHub connection wasn't completed"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "This connection request expired. Select Connect GitHub to try again.",
      ),
    ).toBeInTheDocument();
  });

  it("does not reflect an unknown callback value", () => {
    render(<ConnectGithubCard callbackError="attacker-controlled-text" />);

    expect(
      screen.getByText(
        "GitHub couldn't complete the connection. Select Connect GitHub to try again.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("attacker-controlled-text"),
    ).not.toBeInTheDocument();
  });
});
