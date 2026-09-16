import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ActiveSessionsPanel } from "@/features/sessions/components/active-sessions-panel";
import type { SessionView } from "@/features/sessions/types";

const sessionsState: {
  sessions: SessionView[];
  loading: boolean;
  error: boolean;
} = { sessions: [], loading: false, error: false };
const revoke = vi.fn();
const signOutOthers = vi.fn();
vi.mock("@/features/sessions/hooks/use-active-sessions", () => ({
  useActiveSessions: () => ({
    ...sessionsState,
    revoke,
    revoking: null,
    signOutOthers,
    signingOutOthers: false,
    refetch: vi.fn(),
  }),
}));

beforeEach(() => {
  sessionsState.sessions = [];
  sessionsState.loading = false;
  sessionsState.error = false;
  revoke.mockReset();
  signOutOthers.mockReset();
});

describe("ActiveSessionsPanel", () => {
  it("lists sessions and marks the current one", () => {
    sessionsState.sessions = [
      {
        id: "session-current",
        current: true,
        userAgent: "Chrome on macOS",
        authenticatedAt: null,
      },
      {
        id: "session-other",
        current: false,
        userAgent: "Safari on iOS",
        authenticatedAt: null,
      },
    ];
    render(<ActiveSessionsPanel />);

    expect(screen.getByText("Chrome on macOS")).toBeInTheDocument();
    expect(screen.getByText("This device")).toBeInTheDocument();
    expect(screen.getByText("Safari on iOS")).toBeInTheDocument();
  });

  it("shows an empty state with no sessions", () => {
    render(<ActiveSessionsPanel />);
    expect(screen.getByText("No active sessions")).toBeInTheDocument();
  });

  it("shows a generic error state on failure", () => {
    sessionsState.error = true;
    render(<ActiveSessionsPanel />);
    expect(
      screen.getByText("Couldn't load active sessions"),
    ).toBeInTheDocument();
  });

  it("disables sign-out-others when there are no other sessions", () => {
    sessionsState.sessions = [
      {
        id: "session-current",
        current: true,
        userAgent: "Chrome on macOS",
        authenticatedAt: null,
      },
    ];
    render(<ActiveSessionsPanel />);
    expect(
      screen.getByRole("button", { name: "Sign out other sessions" }),
    ).toBeDisabled();
  });

  it("never renders a revoke control for the current session's row", () => {
    sessionsState.sessions = [
      {
        id: "session-current",
        current: true,
        userAgent: "Chrome on macOS",
        authenticatedAt: null,
      },
    ];
    render(<ActiveSessionsPanel />);
    expect(
      screen.queryByRole("button", { name: "Sign out" }),
    ).not.toBeInTheDocument();
  });

  it("confirming sign-out-others calls the hook's signOutOthers", async () => {
    sessionsState.sessions = [
      {
        id: "session-current",
        current: true,
        userAgent: "Chrome on macOS",
        authenticatedAt: null,
      },
      {
        id: "session-other",
        current: false,
        userAgent: "Safari on iOS",
        authenticatedAt: null,
      },
    ];
    signOutOthers.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<ActiveSessionsPanel />);

    await user.click(
      screen.getByRole("button", { name: "Sign out other sessions" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Sign out other sessions" }),
    );

    expect(signOutOthers).toHaveBeenCalled();
  });

  it("confirming a row's sign-out calls revoke with that session's id", async () => {
    sessionsState.sessions = [
      {
        id: "session-current",
        current: true,
        userAgent: "Chrome on macOS",
        authenticatedAt: null,
      },
      {
        id: "session-other",
        current: false,
        userAgent: "Safari on iOS",
        authenticatedAt: null,
      },
    ];
    revoke.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<ActiveSessionsPanel />);

    await user.click(
      screen.getByRole("button", {
        name: "Sign out Safari on iOS, —, last active —",
      }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getAllByRole("button", { name: "Sign out" })[0],
    );

    expect(revoke).toHaveBeenCalledWith("session-other");
  });

  it("names the row sign-out control after device, location, and last active (w4/084)", () => {
    sessionsState.sessions = [
      {
        id: "session-current",
        current: true,
        userAgent: "Chrome on macOS",
        authenticatedAt: null,
      },
      {
        id: "session-other",
        current: false,
        userAgent: "curl/8.7.1",
        location: "US",
        authenticatedAt: "2026-07-08T00:00:00Z",
      },
      {
        id: "session-unknown",
        current: false,
        authenticatedAt: null,
      },
    ];
    vi.setSystemTime(new Date("2026-07-08T00:18:00Z"));
    render(<ActiveSessionsPanel />);

    expect(
      screen.getByRole("button", {
        name: "Sign out curl/8.7.1, US, last active 18m",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Sign out Unknown device, —, last active —",
      }),
    ).toBeInTheDocument();
  });
});
