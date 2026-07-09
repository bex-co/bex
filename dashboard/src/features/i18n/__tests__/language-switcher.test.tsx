import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n/init";
import { persistLanguage } from "@/i18n";
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

    expect(
      screen.getByRole("menuitem", { name: "English" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "中文" })).toBeInTheDocument();
  });

  it("marks the active language with a check", async () => {
    const user = userEvent.setup();
    render(<LanguageSwitcher />);

    await user.click(screen.getByRole("button", { name: "Change language" }));

    const activeItem = screen.getByRole("menuitem", { name: "English" });
    expect(activeItem.querySelector("svg")).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "中文" }).querySelector("svg"),
    ).toBeNull();
  });

  it("selecting a language changes i18next's language and persists it", async () => {
    const changeLanguageSpy = vi.spyOn(i18n, "changeLanguage");
    const user = userEvent.setup();
    render(<LanguageSwitcher />);

    await user.click(screen.getByRole("button", { name: "Change language" }));
    await user.click(screen.getByRole("menuitem", { name: "中文" }));

    expect(changeLanguageSpy).toHaveBeenCalledWith("zh");
    expect(persistLanguage).toHaveBeenCalledWith("zh");
  });
});
