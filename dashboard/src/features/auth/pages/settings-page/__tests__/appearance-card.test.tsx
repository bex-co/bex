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
// Theme and High Contrast Mode, in Render's order, saved on the account. The
// two themes are Render's `RowInput` rows (w3/097): Edit, then Cancel or Save
// changes, and only Save changes writes.

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
    <button type="button" onClick={() => void setTheme("light")}>
      Account menu: Light
    </button>
  );
}

const SYSTEM: ThemeSettings = {
  themeSetting: "SYSTEM",
  logThemeSetting: "MATCH_APP",
};
const LIGHT: ThemeSettings = {
  themeSetting: "LIGHT",
  logThemeSetting: "MATCH_APP",
};

function Page({
  settings,
  showCard = true,
}: {
  settings: ThemeSettings;
  showCard?: boolean;
}) {
  return (
    <ThemeProvider>
      <Account settings={settings} />
      {showCard ? <AppearanceCard /> : null}
      <AccountMenuTheme />
    </ThemeProvider>
  );
}

function renderCard(settings: ThemeSettings = SYSTEM) {
  return render(<Page settings={settings} />);
}

const rowNames = (card: HTMLElement) =>
  [
    ...card.querySelectorAll(":scope > div > div:first-child > p:first-child"),
  ].map((p) => p.textContent);

/** A theme row: its select and the buttons under it. */
function row(name: "Dashboard Theme" | "Log Explorer Theme") {
  const group = screen.getByRole("group", { name });
  const button = (label: string) =>
    within(group).queryByRole("button", { name: label });
  return {
    select: within(group).getByRole("combobox", { name }),
    edit: () => button("Edit"),
    cancel: () => button("Cancel"),
    save: () =>
      button("Save changes") ??
      within(group).queryByRole("button", { name: "Saving" }),
  };
}

const html = () => document.documentElement;

async function pick(
  user: ReturnType<typeof userEvent.setup>,
  select: HTMLElement,
  option: string,
) {
  await user.click(select);
  await user.click(screen.getByRole("option", { name: option }));
}

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

  it("reads each theme row as its saved value and an Edit button, with nothing to pick until Edit", () => {
    renderCard({
      themeSetting: "DARK_HIGH_CONTRAST",
      logThemeSetting: "ALWAYS_LIGHT",
    });
    const theme = row("Dashboard Theme");
    // A twin reads as its base.
    expect(theme.select).toHaveTextContent("Dark");
    expect(theme.select).toBeDisabled();
    expect(theme.edit()).toBeEnabled();
    expect(theme.cancel()).toBeNull();
    expect(theme.save()).toBeNull();
    const log = row("Log Explorer Theme");
    expect(log.select).toHaveTextContent("Light");
    expect(log.select).toBeDisabled();
    expect(log.edit()).toBeEnabled();
    expect(
      screen.getByRole("switch", { name: "High Contrast Mode" }),
    ).toBeChecked();
  });

  it("offers Dashboard Theme behind Edit, with System the default, and Save changes only once the pick differs", async () => {
    const user = userEvent.setup();
    renderCard();
    const theme = row("Dashboard Theme");
    expect(theme.select).toHaveTextContent("System (Default)");
    await user.click(theme.edit()!);
    expect(theme.select).toBeEnabled();
    expect(theme.select).toHaveFocus();
    expect(theme.edit()).toBeNull();
    expect(theme.cancel()).toBeEnabled();
    expect(theme.save()).toBeDisabled();
    await user.click(theme.select);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["System (Default)", "Light", "Dark"]);
    await user.click(screen.getByRole("option", { name: "Dark" }));
    expect(theme.save()).toBeEnabled();
    await pick(user, theme.select, "System (Default)");
    expect(theme.save()).toBeDisabled();
  });

  it("previews a Dashboard Theme pick on the page and writes nothing until Save changes", async () => {
    const user = userEvent.setup();
    renderCard(LIGHT);
    const theme = row("Dashboard Theme");
    expect(html()).toHaveClass("light");
    await user.click(theme.edit()!);
    await pick(user, theme.select, "Dark");
    expect(html()).toHaveClass("dark");
    expect(theme.select).toHaveTextContent("Dark");
    expect(write).not.toHaveBeenCalled();
    // The preview is not the browser's copy either: a reload paints Light.
    expect(localStorage.getItem("bex-user-settings")).toContain('"LIGHT"');

    await user.click(theme.save()!);
    expect(write).toHaveBeenCalledTimes(1);
    expect(write).toHaveBeenLastCalledWith({ themeSetting: "DARK" });
    expect(html()).toHaveClass("dark");
    expect(theme.select).toBeDisabled();
    expect(theme.select).toHaveTextContent("Dark");
    expect(theme.edit()).toHaveFocus();
    expect(localStorage.getItem("bex-user-settings")).toContain('"DARK"');
  });

  it("puts the saved theme back on Cancel, writing nothing", async () => {
    const user = userEvent.setup();
    renderCard(LIGHT);
    const theme = row("Dashboard Theme");
    await user.click(theme.edit()!);
    await pick(user, theme.select, "Dark");
    expect(html()).toHaveClass("dark");
    await user.click(theme.cancel()!);
    expect(html()).toHaveClass("light");
    expect(html()).not.toHaveClass("dark");
    expect(theme.select).toHaveTextContent("Light");
    expect(theme.select).toBeDisabled();
    expect(theme.edit()).toHaveFocus();
    expect(write).not.toHaveBeenCalled();
  });

  it("puts the saved theme back when the page is left mid-edit", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Page settings={LIGHT} />);
    const theme = row("Dashboard Theme");
    await user.click(theme.edit()!);
    await pick(user, theme.select, "Dark");
    expect(html()).toHaveClass("dark");
    rerender(<Page settings={LIGHT} showCard={false} />);
    expect(html()).toHaveClass("light");
    expect(html()).not.toHaveClass("dark");
    expect(write).not.toHaveBeenCalled();
  });

  it("reads Saving while the account answers, then closes the row", async () => {
    const user = userEvent.setup();
    let answer: (saved: object) => void = () => undefined;
    write.mockImplementation(
      () =>
        new Promise((resolve) => {
          answer = resolve;
        }),
    );
    renderCard(LIGHT);
    const theme = row("Dashboard Theme");
    await user.click(theme.edit()!);
    await pick(user, theme.select, "Dark");
    await user.click(theme.save()!);
    const saving = within(
      screen.getByRole("group", { name: "Dashboard Theme" }),
    ).getByRole("button", { name: "Saving" });
    expect(saving).toBeDisabled();
    expect(theme.cancel()).toBeDisabled();
    expect(theme.select).toBeDisabled();
    // High Contrast Mode still waits: the theme is still being edited.
    expect(
      screen.getByRole("switch", { name: "High Contrast Mode" }),
    ).toBeDisabled();
    await act(async () => answer({ themeSetting: "DARK" }));
    expect(theme.edit()).toBeEnabled();
    expect(theme.save()).toBeNull();
    expect(html()).toHaveClass("dark");
    expect(
      screen.getByRole("switch", { name: "High Contrast Mode" }),
    ).toBeEnabled();
  });

  it("keeps the row open when the account refuses the save, the saved theme back on the page", async () => {
    const user = userEvent.setup();
    write.mockRejectedValue(new Error("refused"));
    renderCard(LIGHT);
    const theme = row("Dashboard Theme");
    await user.click(theme.edit()!);
    await pick(user, theme.select, "Dark");
    await user.click(theme.save()!);
    expect(write).toHaveBeenCalledTimes(1);
    expect(html()).toHaveClass("light");
    // The pick is still there to save again.
    expect(theme.select).toHaveTextContent("Dark");
    expect(theme.save()).toBeEnabled();
    expect(theme.cancel()).toBeEnabled();
  });

  it("disables High Contrast Mode's switch while Dashboard Theme is being edited", async () => {
    const user = userEvent.setup();
    renderCard(LIGHT);
    const toggle = screen.getByRole("switch", { name: "High Contrast Mode" });
    expect(toggle).toBeEnabled();
    await user.click(row("Dashboard Theme").edit()!);
    expect(toggle).toBeDisabled();
    await user.click(row("Dashboard Theme").cancel()!);
    expect(toggle).toBeEnabled();
    // Editing Log Explorer Theme leaves it alone, as Render's does.
    await user.click(row("Log Explorer Theme").edit()!);
    expect(toggle).toBeEnabled();
  });

  it("offers Log Explorer Theme behind Edit, with Match Dashboard the default, and writes it only on Save changes", async () => {
    const user = userEvent.setup();
    renderCard(LIGHT);
    const log = row("Log Explorer Theme");
    expect(log.select).toHaveTextContent("Match Dashboard (Default)");
    await user.click(log.edit()!);
    await user.click(log.select);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["Match Dashboard (Default)", "Light", "Dark"]);
    await user.click(screen.getByRole("option", { name: "Dark" }));
    expect(log.select).toHaveTextContent("Dark");
    // Nothing on the settings page is a log explorer: no preview, no write.
    expect(html()).toHaveClass("light");
    expect(write).not.toHaveBeenCalled();
    await user.click(log.save()!);
    expect(write).toHaveBeenCalledTimes(1);
    expect(write).toHaveBeenLastCalledWith({ logThemeSetting: "ALWAYS_DARK" });
    expect(log.select).toHaveTextContent("Dark");
    expect(log.edit()).toBeEnabled();
  });

  it("leaves Log Explorer Theme as saved on Cancel", async () => {
    const user = userEvent.setup();
    renderCard(LIGHT);
    const log = row("Log Explorer Theme");
    await user.click(log.edit()!);
    await pick(user, log.select, "Light");
    await user.click(log.cancel()!);
    expect(log.select).toHaveTextContent("Match Dashboard (Default)");
    expect(write).not.toHaveBeenCalled();
  });

  it("shows a change made in the account menu, which still writes at once", async () => {
    const user = userEvent.setup();
    renderCard({
      themeSetting: "DARK_HIGH_CONTRAST",
      logThemeSetting: "ALWAYS_LIGHT",
    });
    const theme = row("Dashboard Theme");
    await user.click(
      screen.getByRole("button", { name: "Account menu: Light" }),
    );
    await act(async () => undefined);
    expect(theme.select).toHaveTextContent("Light");
    // The theme changed, High Contrast Mode stayed on.
    expect(write).toHaveBeenLastCalledWith({
      themeSetting: "LIGHT_HIGH_CONTRAST",
    });
  });

  it("turns High Contrast Mode on and off as the current theme's twin, Dashboard Theme unchanged", async () => {
    const user = userEvent.setup();
    renderCard(LIGHT);
    const toggle = screen.getByRole("switch", { name: "High Contrast Mode" });
    const label = toggle.closest("label") as HTMLElement;
    expect(toggle).not.toBeChecked();
    expect(within(label).getByText("Disabled")).toBeInTheDocument();
    await user.click(toggle);
    expect(toggle).toBeChecked();
    expect(within(label).getByText("Enabled")).toBeInTheDocument();
    expect(document.documentElement).toHaveClass("light", "high-contrast");
    expect(write).toHaveBeenLastCalledWith({
      themeSetting: "LIGHT_HIGH_CONTRAST",
    });
    expect(row("Dashboard Theme").select).toHaveTextContent("Light");
    await user.click(toggle);
    expect(document.documentElement).not.toHaveClass("high-contrast");
    expect(write).toHaveBeenLastCalledWith({ themeSetting: "LIGHT" });
  });

  it("keeps High Contrast Mode across a saved Dashboard Theme", async () => {
    const user = userEvent.setup();
    renderCard({
      themeSetting: "LIGHT_HIGH_CONTRAST",
      logThemeSetting: "MATCH_APP",
    });
    const theme = row("Dashboard Theme");
    await user.click(theme.edit()!);
    await pick(user, theme.select, "Dark");
    expect(html()).toHaveClass("dark", "high-contrast");
    await user.click(theme.save()!);
    expect(write).toHaveBeenLastCalledWith({
      themeSetting: "DARK_HIGH_CONTRAST",
    });
  });

  it("has a skeleton with the ready card's three rows, in its order, each control in the same boxes", () => {
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
    // Each row's control slot holds the same boxes: a theme row's select
    // over its actions, High Contrast Mode's switch line.
    const controls = (el: Element) =>
      [...el.querySelectorAll(":scope > div > div:last-child > *")].map(
        (control) => ({
          control: control.className,
          parts: [...control.children].map((part) =>
            part.className.split(" ").filter((c) => /^(sm:)?[wh]-/.test(c)),
          ),
        }),
      );
    const ready = controls(card);
    const pending = controls(skeleton);
    expect(pending.map((c) => c.control)).toEqual(ready.map((c) => c.control));
    // The selects and the action rows have the same width and height.
    for (const index of [0, 1]) {
      expect(pending[index].parts).toEqual(ready[index].parts);
    }
  });
});
