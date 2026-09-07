import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { INVITE_TOKEN_STORAGE_KEY } from "@/common/lib/invite-token";
import { rememberInviteReturn } from "../invite-return";

const state = vi.hoisted(() => ({
  details: {
    workspaceId: "tea-target",
    workspaceName: "Acme",
    role: "DEVELOPER",
    inviterEmail: "alex@example.com",
    alreadyMember: false,
  },
  loading: false,
  errorKey: null as string | null,
  busy: false,
  joined: false,
  retryable: true,
  accept: vi.fn(),
  retry: vi.fn(),
}));
vi.mock("@/features/team/hooks/use-invite-redemption", () => ({
  useInviteRedemption: () => state,
}));
vi.mock("@tanstack/react-router", () => ({
  useLocation: () => ({
    href: window.location.pathname + window.location.search,
  }),
  Link: ({
    to,
    search,
    children,
  }: {
    to: string;
    search: { next?: string };
    children: React.ReactNode;
  }) => <a href={`${to}?next=${search.next}`}>{children}</a>,
}));
import { InviteFallbackPage } from "../invite-fallback-page";
const TOKEN = "0123456789abcdef0123456789abcdef";

beforeEach(() => {
  window.sessionStorage.clear();
  window.history.replaceState(null, "", `/invite#invite=${TOKEN}`);
  state.busy = false;
  state.joined = false;
  state.errorKey = null;
  vi.clearAllMocks();
});

describe("invitation page", () => {
  it("offers sign-up and sign-in while preserving the token outside auth URLs", async () => {
    const navigate = vi.fn();
    render(<InviteFallbackPage authenticated={false} continueTo={navigate} />);
    expect(
      await screen.findByRole("link", { name: "Sign up" }),
    ).toHaveAttribute("href", "/auth/sign-up?next=/invite");
    expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute(
      "href",
      "/auth/login?next=/invite",
    );
    expect(window.location.hash).toBe("");
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(TOKEN);
    expect(navigate).not.toHaveBeenCalled();
  });
  it("shows workspace, inviter, role and signed-in account before accepting", async () => {
    render(
      <InviteFallbackPage
        authenticated
        email="different@example.com"
        continueTo={vi.fn()}
      />,
    );
    expect(
      await screen.findByRole("heading", { name: "Join Acme" }),
    ).toBeVisible();
    expect(screen.getByText("Invited by alex@example.com")).toBeVisible();
    expect(
      screen.getByText("Signed in as different@example.com"),
    ).toBeVisible();
    expect(screen.getByText("You’ll join as a Developer.")).toBeVisible();
    expect(state.accept).not.toHaveBeenCalled();
    await userEvent.click(
      screen.getByRole("button", { name: "Join workspace" }),
    );
    expect(state.accept).toHaveBeenCalledOnce();
  });
  it("offers Open for a membership already established during login", async () => {
    state.joined = true;
    render(<InviteFallbackPage authenticated continueTo={vi.fn()} />);
    expect(
      await screen.findByRole("button", { name: "Open workspace" }),
    ).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Join workspace" }),
    ).not.toBeInTheDocument();
  });
  it("Not now clears only local intent and returns to the previous page", async () => {
    rememberInviteReturn("/workspace/settings");
    const navigate = vi.fn();
    render(<InviteFallbackPage authenticated continueTo={navigate} />);
    await userEvent.click(
      await screen.findByRole("button", { name: "Not now" }),
    );
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
    expect(navigate).toHaveBeenCalledWith("/workspace/settings");
    expect(state.accept).not.toHaveBeenCalled();
  });
  it("rejects malformed links and cannot fall back to an older pending invitation", async () => {
    window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, TOKEN);
    window.history.replaceState(null, "", "/invite?invite=invalid");
    render(<InviteFallbackPage authenticated continueTo={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        "invalid or was revoked",
      ),
    );
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Join workspace" }),
    ).not.toBeInTheDocument();
  });
});
