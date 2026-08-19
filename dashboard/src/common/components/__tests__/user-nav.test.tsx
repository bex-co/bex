import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import i18n from "@/i18n/init";
import { UserNav } from "@/common/components/user-nav";

const { navigate, invalidate, endBrowserSession } = vi.hoisted(() => ({
  navigate: vi.fn(),
  invalidate: vi.fn(),
  endBrowserSession: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
  useRouter: () => ({ invalidate }),
}));

vi.mock("@/common/lib/ory/logout", () => ({
  endBrowserSession,
}));

vi.mock("@/common/providers/theme-provider", () => ({
  useTheme: () => ({ theme: "light", setTheme: vi.fn() }),
}));

vi.mock("@/common/hooks/use-mobile", () => ({
  useIsMobile: () => false,
}));

vi.mock("@/common/hooks/use-root-context", () => ({
  useRootContext: () => ({
    session: {
      identity: { traits: { email: "ada@example.com", name: "Ada" } },
    },
  }),
}));

describe("UserNav logout", () => {
  beforeEach(() => {
    navigate.mockReset();
    invalidate.mockReset().mockResolvedValue(undefined);
    endBrowserSession.mockReset().mockResolvedValue(undefined);
  });

  it("signs out immediately from the avatar menu without visiting /auth/logout", async () => {
    const user = userEvent.setup();
    render(
      <I18nextProvider i18n={i18n}>
        <UserNav />
      </I18nextProvider>,
    );

    await user.click(screen.getByRole("button"));
    await user.click(await screen.findByRole("menuitem", { name: "Log out" }));

    await waitFor(() => {
      expect(endBrowserSession).toHaveBeenCalledOnce();
    });
    expect(invalidate).toHaveBeenCalledOnce();
    expect(navigate).toHaveBeenCalledWith({
      to: "/auth/login",
      search: {
        next: undefined,
        flow: undefined,
        login_challenge: undefined,
        aal: undefined,
      },
    });
    expect(navigate).not.toHaveBeenCalledWith(
      expect.objectContaining({ to: "/auth/logout" }),
    );
  });
});
