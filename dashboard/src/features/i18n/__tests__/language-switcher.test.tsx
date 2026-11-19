import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n/init";
import { LANGUAGE_NAMES, SUPPORTED_LANGUAGES, persistLanguage } from "@/i18n";
import { LanguageSwitcher } from "../language-switcher";

vi.mock("@/i18n", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/i18n")>();
  return { ...actual, persistLanguage: vi.fn() };
});

describe("LanguageSwitcher", () => {
  afterEach(async () => {
    vi.clearAllMocks();
    await i18n.changeLanguage("en");
  });

  it("lists every supported language by native name", async () => {
    const user = userEvent.setup();
    render(<LanguageSwitcher />);

    await user.click(screen.getByRole("button", { name: "Change language" }));

    // The bex.co locale set, each in its own script.
    expect(
      screen.getAllByRole("menuitem").map((item) => item.textContent),
    ).toEqual(SUPPORTED_LANGUAGES.map((lang) => LANGUAGE_NAMES[lang]));
    for (const name of [
      "English",
      "简体中文",
      "한국어",
      "日本語",
      "Español",
      "Português",
      "Русский",
      "Deutsch",
    ]) {
      expect(screen.getByRole("menuitem", { name })).toBeInTheDocument();
    }
  });

  it("marks the active language with a check", async () => {
    const user = userEvent.setup();
    render(<LanguageSwitcher />);

    await user.click(screen.getByRole("button", { name: "Change language" }));

    const activeItem = screen.getByRole("menuitem", { name: "English" });
    expect(activeItem.querySelector("svg")).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "简体中文" }).querySelector("svg"),
    ).toBeNull();
  });

  it("selecting a language changes i18next's language and persists it", async () => {
    const changeLanguageSpy = vi.spyOn(i18n, "changeLanguage");
    const user = userEvent.setup();
    render(<LanguageSwitcher />);

    await user.click(screen.getByRole("button", { name: "Change language" }));
    await user.click(screen.getByRole("menuitem", { name: "简体中文" }));

    expect(changeLanguageSpy).toHaveBeenCalledWith("zh");
    expect(persistLanguage).toHaveBeenCalledWith("zh");
    expect(vi.mocked(persistLanguage).mock.invocationCallOrder[0]).toBeLessThan(
      changeLanguageSpy.mock.invocationCallOrder[0],
    );
  });

  it("selecting a lazily loaded language registers its catalog first", async () => {
    // Only en (entry bundle) and zh (test setup) are preloaded: de's catalog is
    // its own async chunk, which switchLanguage must load before the switch or
    // the UI renders the en fallback under i18n.language === "de".
    const user = userEvent.setup();
    render(<LanguageSwitcher />);

    await user.click(screen.getByRole("button", { name: "Change language" }));
    await user.click(screen.getByRole("menuitem", { name: "Deutsch" }));

    // Dynamic import of resources-de transforms ~35 locale modules on first
    // load, which exceeds Testing Library's 1000ms default under parallel runs.
    await waitFor(() => expect(i18n.language).toBe("de"), { timeout: 5_000 });
    expect(i18n.hasResourceBundle("de", "translation")).toBe(true);
    expect(i18n.t("common.cancel")).toBe("Abbrechen");
    expect(persistLanguage).toHaveBeenCalledWith("de");
  });
});
