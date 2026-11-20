import { useContext, useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeProvider } from "@/common/providers/theme-provider";
import { ThemeProviderContext } from "@/common/providers/theme-provider/context";
import type { ThemeSettings } from "@/common/providers/theme-provider/settings";
import { memoryStorage } from "@/test/mocks/storage";
import { AppearanceCard } from "../appearance-card";
import { AppearanceCardSkeleton } from "../appearance-card-skeleton";

// Render's Appearance (w3/m528, w3/m531): Dashboard Theme, Log Explorer
// Theme and High Contrast Mode, in Render's order, saved on the account.

const write = vi.fn<(patch: Partial<ThemeSettings>) => Promise<object>>();

/** The signed-in shell's binding, with the account's stored settings. */
function Account({ settings }: { settings: ThemeSettings }) {
  const { bindAccount } = useContext(ThemeProviderContext);
  useEffect(() => {
    bindAccount({ settings, write });
  }, [bindAccount, settings]);
  return null;
}

// The account menu's theme submenu changes the same setting.
function AccountMenuTheme() {
  const { setTheme } = useContext(ThemeProviderContext);
  return (
    <button type="button" onClick={() => setTheme("light")}>
      Account menu: Light
    </button>
  );
}

const SYSTEM: ThemeSettings = {
  themeSetting: "SYSTEM",
  logThemeSetting: "MATCH_APP",
};

function renderCard(settings: ThemeSettings = SYSTEM) {
  return render(
    <ThemeProvider>
      <Account settings={settings} />
      <AppearanceCard />
      <AccountMenuTheme />
    </ThemeProvider>,
  );
}

const rowNames = (card: HTMLElement) =>
  [
    ...card.querySelectorAll(":scope > div > div:first-child > p:first-child"),
  ].map((p) => p.textContent);

beforeEach(() => {
  vi.stubGlobal("localStorage", memoryStorage());
  write.mockReset().mockImplementation(async (patch) => patch);
});
afterEach(() => {
  vi.unstubAllGlobals();
  document.documentElement.classList.remove("light", "dark", "high-contrast");
});

describe("Account Settings ▸ Appearance (w3/m528, w3/m531)", () => {
  it("lists Render's three settings in Render's order", () => {
    renderCard();
    const card = screen.getByTestId("appearance-card");
    expect(rowNames(card)).toEqual([
      "Dashboard Theme",
      "Log Explorer Theme",
      "High Contrast Mode",
    ]);
    expect(card).toHaveTextContent(
      "Increase the visibility of interactive elements.",
    );
  });

  it("offers Dashboard Theme with System the default, and writes a choice to the account", async () => {
    const user = userEvent.setup();
    renderCard();
    const select = screen.getByRole("combobox", { name: "Dashboard Theme" });
    expect(select).toHaveTextContent("System (Default)");
    await user.click(select);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["System (Default)", "Light", "Dark"]);
    await user.click(screen.getByRole("option", { name: "Dark" }));
    expect(select).toHaveTextContent("Dark");
    expect(document.documentElement).toHaveClass("dark");
    expect(write).toHaveBeenLastCalledWith({ themeSetting: "DARK" });
  });

  it("shows the account's stored settings, and a change made in the account menu", async () => {
    const user = userEvent.setup();
    renderCard({
      themeSetting: "DARK_HIGH_CONTRAST",
      logThemeSetting: "ALWAYS_LIGHT",
    });
    const theme = screen.getByRole("combobox", { name: "Dashboard Theme" });
    // A twin reads as its base.
    expect(theme).toHaveTextContent("Dark");
    expect(
      screen.getByRole("combobox", { name: "Log Explorer Theme" }),
    ).toHaveTextContent("Light");
    expect(
      screen.getByRole("switch", { name: "High Contrast Mode" }),
    ).toBeChecked();
    await user.click(
      screen.getByRole("button", { name: "Account menu: Light" }),
    );
    await act(async () => undefined);
    expect(theme).toHaveTextContent("Light");
    // The theme changed, High Contrast Mode stayed on.
    expect(write).toHaveBeenLastCalledWith({
      themeSetting: "LIGHT_HIGH_CONTRAST",
    });
  });

  it("offers Log Explorer Theme with Match Dashboard the default, and writes a choice", async () => {
    const user = userEvent.setup();
    renderCard();
    const select = screen.getByRole("combobox", { name: "Log Explorer Theme" });
    expect(select).toHaveTextContent("Match Dashboard (Default)");
    await user.click(select);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["Match Dashboard (Default)", "Light", "Dark"]);
    await user.click(screen.getByRole("option", { name: "Dark" }));
    expect(select).toHaveTextContent("Dark");
    expect(write).toHaveBeenLastCalledWith({ logThemeSetting: "ALWAYS_DARK" });
  });

  it("turns High Contrast Mode on and off as the current theme's twin, Dashboard Theme unchanged", async () => {
    const user = userEvent.setup();
    renderCard({ themeSetting: "LIGHT", logThemeSetting: "MATCH_APP" });
    const toggle = screen.getByRole("switch", { name: "High Contrast Mode" });
    const row = toggle.closest("label") as HTMLElement;
    expect(toggle).not.toBeChecked();
    expect(within(row).getByText("Disabled")).toBeInTheDocument();
    await user.click(toggle);
    expect(toggle).toBeChecked();
    expect(within(row).getByText("Enabled")).toBeInTheDocument();
    expect(document.documentElement).toHaveClass("light", "high-contrast");
    expect(write).toHaveBeenLastCalledWith({
      themeSetting: "LIGHT_HIGH_CONTRAST",
    });
    expect(
      screen.getByRole("combobox", { name: "Dashboard Theme" }),
    ).toHaveTextContent("Light");
    await user.click(toggle);
    expect(document.documentElement).not.toHaveClass("high-contrast");
    expect(write).toHaveBeenLastCalledWith({ themeSetting: "LIGHT" });
  });

  it("has a skeleton with the ready card's three rows, in its order", () => {
    render(
      <>
        <AppearanceCardSkeleton />
        <ThemeProvider>
          <AppearanceCard />
        </ThemeProvider>
      </>,
    );
    const skeleton = screen.getByTestId("appearance-card-skeleton");
    const card = screen.getByTestId("appearance-card");
    expect(rowNames(skeleton)).toEqual(rowNames(card));
    expect(skeleton.className).toBe(card.className);
    // Each row's control sits in the same h-9 slot.
    const slots = (el: Element) =>
      [...el.querySelectorAll(":scope > div > div:last-child")].map(
        (slot) => slot.className,
      );
    expect(slots(skeleton)).toEqual(slots(card));
  });
});
