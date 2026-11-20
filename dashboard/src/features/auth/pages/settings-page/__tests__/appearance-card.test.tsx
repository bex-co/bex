import { useContext } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeProvider } from "@/common/providers/theme-provider";
import { ThemeProviderContext } from "@/common/providers/theme-provider/context";
import { THEME_STORAGE_KEY } from "@/common/providers/theme-provider/utils";
import { memoryStorage } from "@/test/mocks/storage";
import { AppearanceCard } from "../appearance-card";

// Render's Appearance ▸ Dashboard Theme (w3/m528): the same setting as the
// account menu's theme submenu, which drives the theme context too.
function AccountMenuTheme() {
  const { setTheme } = useContext(ThemeProviderContext);
  return (
    <button type="button" onClick={() => setTheme("light")}>
      Account menu: Light
    </button>
  );
}

// The theme persists in the browser, which is the point here.
beforeEach(() => {
  vi.stubGlobal("localStorage", memoryStorage());
});
afterEach(() => {
  vi.unstubAllGlobals();
  document.documentElement.classList.remove("light", "dark");
});

describe("Account Settings ▸ Appearance (w3/m528)", () => {
  it("offers Render's Dashboard Theme, System marked the default, and changes the dashboard's theme", async () => {
    const user = userEvent.setup();
    render(
      <ThemeProvider>
        <AppearanceCard />
      </ThemeProvider>,
    );
    const select = screen.getByRole("combobox", { name: "Dashboard Theme" });
    expect(select).toHaveTextContent("System (Default)");
    await user.click(select);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["System (Default)", "Light", "Dark"]);
    await user.click(screen.getByRole("option", { name: "Dark" }));
    expect(select).toHaveTextContent("Dark");
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark");
    expect(document.documentElement).toHaveClass("dark");
  });

  it("shows a change made in the account menu", async () => {
    const user = userEvent.setup();
    localStorage.setItem(THEME_STORAGE_KEY, "dark");
    render(
      <ThemeProvider>
        <AppearanceCard />
        <AccountMenuTheme />
      </ThemeProvider>,
    );
    const select = screen.getByRole("combobox", { name: "Dashboard Theme" });
    // The browser's stored choice, read once the page has hydrated.
    expect(select).toHaveTextContent("Dark");
    await user.click(
      screen.getByRole("button", { name: "Account menu: Light" }),
    );
    await act(async () => undefined);
    expect(select).toHaveTextContent("Light");
  });
});
