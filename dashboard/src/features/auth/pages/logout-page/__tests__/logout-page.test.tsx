import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { EMPTY_LOGIN_SEARCH } from "@/common/lib/auth/auth";

// LogoutPage is a state machine driven by endBrowserSession() — mock the
// router hooks and that one call so the test drives status transitions
// directly, with no router mount and no live Kratos (mirrors
// device-confirm-page.test.tsx / consent-page.test.tsx). endBrowserSession's
// own upstream-call behavior is unit-tested in
// common/lib/ory/__tests__/logout.test.ts — this page treats it as a black box.
const navigate = vi.fn();
const invalidate = vi.fn();
vi.mock("@tanstack/react-router", async (orig) => ({
  ...(await orig<typeof import("@tanstack/react-router")>()),
  useNavigate: () => navigate,
  useRouter: () => ({ invalidate }),
}));

const endBrowserSession = vi.fn();
vi.mock("@/common/lib/ory/logout", () => ({
  endBrowserSession: () => endBrowserSession(),
}));

import LogoutPage from "@/features/auth/pages/logout-page";

beforeEach(() => {
  navigate.mockClear();
  invalidate.mockClear();
  endBrowserSession.mockReset();
});

describe("LogoutPage", () => {
  // w10/m8 t003: every status renders inside AuthPageShell, so the language
  // switcher (mounted once by the shell) is present regardless of state.
  it("renders inside AuthPageShell", () => {
    render(<LogoutPage />);

    expect(
      screen.getByRole("button", { name: "Change language" }),
    ).toBeInTheDocument();
  });

  it("requires an explicit click before calling Kratos (codex #12 CSRF logout)", () => {
    render(<LogoutPage />);

    expect(screen.getByText("Sign out?")).toBeInTheDocument();
    expect(endBrowserSession).not.toHaveBeenCalled();
  });

  it("ends the session and leaves for login only after a successful provider response", async () => {
    endBrowserSession.mockResolvedValue(undefined);

    render(<LogoutPage />);
    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));

    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({
        to: "/auth/login",
        search: EMPTY_LOGIN_SEARCH,
      }),
    );
    expect(invalidate).toHaveBeenCalled();
  });

  it("keeps a blocking error with retry when the provider logout fails — never a false success screen (codex #6)", async () => {
    endBrowserSession.mockRejectedValue(new Error("logout request failed: 500"));

    render(<LogoutPage />);
    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));

    await waitFor(() =>
      expect(screen.getByText("Sign-out failed")).toBeInTheDocument(),
    );
    expect(navigate).not.toHaveBeenCalled();

    endBrowserSession.mockClear();
    endBrowserSession.mockResolvedValue(undefined);
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await waitFor(() => expect(endBrowserSession).toHaveBeenCalled());
  });
});
