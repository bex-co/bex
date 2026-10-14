import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  waitFor,
  fireEvent,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import i18n from "@/i18n/init";
import zhResources from "@/i18n/resources-zh";
import { UserNav } from "@/common/components/user-nav";

const { navigate, invalidate, endBrowserSession, setTheme, viewport } =
  vi.hoisted(() => ({
    navigate: vi.fn(),
    invalidate: vi.fn(),
    endBrowserSession: vi.fn(),
    setTheme: vi.fn(),
    viewport: { isMobile: false },
  }));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
  useRouter: () => ({ invalidate }),
}));

vi.mock("@/common/lib/ory/logout", () => ({
  endBrowserSession,
}));

vi.mock("@/common/providers/theme-provider", () => ({
  useTheme: () => ({ theme: "light", setTheme }),
}));

vi.mock("@/common/hooks/use-mobile", () => ({
  useIsMobile: () => viewport.isMobile,
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

describe("UserNav language switch (w6/m103 Bug A)", () => {
  // `test/setup.ts` globally preloads the zh bundle so synchronous
  // `changeLanguage("zh")` works in most tests — which also hides this exact
  // regression. Strip it here so the switch must honor the real lazy-load
  // contract (`ensureLanguage` before `changeLanguage`); restore it afterward
  // so the rest of the suite keeps its eager availability.
  beforeEach(async () => {
    i18n.removeResourceBundle("zh", "translation");
    await i18n.changeLanguage("en");
  });

  afterEach(async () => {
    i18n.addResourceBundle("zh", "translation", zhResources, true, true);
    await i18n.changeLanguage("en");
  });

  it("registers the zh catalog before switching, so the first switch actually applies Chinese", async () => {
    const user = userEvent.setup();
    render(
      <I18nextProvider i18n={i18n}>
        <UserNav />
      </I18nextProvider>,
    );

    await user.click(screen.getByRole("button"));
    // Nested Radix submenu items don't respond to userEvent's synthesized
    // pointer sequence under jsdom (the top-level menu does); dispatch a raw
    // click, which still fires the item's plain React `onClick`.
    fireEvent.click(await screen.findByRole("menuitem", { name: /Language/i }));
    fireEvent.click(await screen.findByRole("menuitem", { name: "中文" }));

    // Post-fix: `ensureLanguage("zh")` loaded the catalog before the switch, so
    // both the bundle is present and the active language is zh. Pre-fix (the
    // regression) `changeLanguage("zh")` ran with no catalog registered, so
    // `hasResourceBundle` stays false and the UI would render the en fallback.
    await waitFor(() => {
      expect(i18n.hasResourceBundle("zh", "translation")).toBe(true);
      expect(i18n.language).toBe("zh");
    });
  });
});

describe("UserNav at narrow widths (w4/302)", () => {
  beforeEach(async () => {
    viewport.isMobile = true;
    navigate.mockReset();
    setTheme.mockReset();
    vi.mocked(localStorage.setItem).mockClear();
    await i18n.changeLanguage("en");
  });

  afterEach(async () => {
    viewport.isMobile = false;
    await i18n.changeLanguage("en");
  });

  function renderNav() {
    return render(
      <I18nextProvider i18n={i18n}>
        <UserNav />
      </I18nextProvider>,
    );
  }

  it("opens the avatar menu instead of navigating to Settings", async () => {
    const user = userEvent.setup();
    renderNav();

    const avatar = screen.getByRole("button");
    expect(avatar).toHaveAttribute("aria-haspopup", "menu");
    await user.click(avatar);

    expect(await screen.findByRole("menu")).toBeInTheDocument();
    expect(navigate).not.toHaveBeenCalled();
    // Account Settings is still one tap away, with its original target.
    await user.click(
      screen.getByRole("menuitem", { name: "Account Settings" }),
    );
    expect(navigate).toHaveBeenCalledWith({ to: "/settings" });
  });

  it("lists every theme and language inline (no hover-only flyout)", async () => {
    const user = userEvent.setup();
    renderNav();
    await user.click(screen.getByRole("button"));

    const theme = await screen.findByRole("group", { name: "Theme" });
    const language = screen.getByRole("group", { name: "Language" });
    for (const name of ["Light", "Dark", "System"]) {
      expect(within(theme).getByRole("menuitem", { name })).toBeVisible();
    }
    for (const name of ["English", "中文"]) {
      expect(
        within(language).getByRole("menuitem", { name: new RegExp(name) }),
      ).toBeVisible();
    }
    // Inline groups replace the desktop submenu triggers on narrow widths.
    expect(
      screen.queryByRole("menuitem", { name: /^Theme/ }),
    ).not.toBeInTheDocument();
  });

  it("applies a theme with a tap and stays on the page", async () => {
    const user = userEvent.setup();
    renderNav();
    await user.click(screen.getByRole("button"));
    await user.click(await screen.findByRole("menuitem", { name: "Dark" }));

    expect(setTheme).toHaveBeenCalledWith("dark");
    expect(navigate).not.toHaveBeenCalled();
  });

  it("switches and persists the language from the keyboard", async () => {
    const user = userEvent.setup();
    renderNav();
    screen.getByRole("button").focus();
    await user.keyboard("{Enter}");
    const zh = await screen.findByRole("menuitem", { name: "中文" });
    zh.focus();
    await user.keyboard("{Enter}");

    await waitFor(() => expect(i18n.language).toBe("zh"));
    // persistLanguage writes the reload-surviving copy (cookie + storage).
    expect(localStorage.setItem).toHaveBeenCalledWith("i18nextLng", "zh");
    expect(navigate).not.toHaveBeenCalled();
  });
});
