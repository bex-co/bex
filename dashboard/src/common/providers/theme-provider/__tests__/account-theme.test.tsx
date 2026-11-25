import { useContext } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { memoryStorage } from "@/test/mocks/storage";
import { ThemeProvider } from "../index";
import { ThemeProviderContext } from "../context";
import { SETTINGS_STORAGE_KEY, type ThemeSettings } from "../settings";
import type { AccountThemeBinding, ThemeProviderContextType } from "../type";
import { THEME_STORAGE_KEY } from "../utils";

// The theme follows the account (w3/m531): the signed-in shell binds it, its
// stored settings win over the browser's copy, every change is written to
// it, a refused write puts the previous theme back, and another tab follows.

let theme: ThemeProviderContextType;
function Capture() {
  theme = useContext(ThemeProviderContext);
  return null;
}

const root = () => document.documentElement.classList;
const stored = (): ThemeSettings | null =>
  JSON.parse(localStorage.getItem(SETTINGS_STORAGE_KEY) ?? "null");

function renderProvider() {
  return render(
    <ThemeProvider>
      <Capture />
    </ThemeProvider>,
  );
}

/** A binding whose writes the test settles. */
function account(settings: ThemeSettings) {
  const writes: {
    patch: Partial<ThemeSettings>;
    resolve: (saved: Partial<ThemeSettings>) => void;
    reject: (cause: Error) => void;
  }[] = [];
  const binding: AccountThemeBinding = {
    settings,
    write: (patch) =>
      new Promise((resolve, reject) => writes.push({ patch, resolve, reject })),
  };
  return { binding, writes };
}

beforeEach(() => {
  vi.stubGlobal("localStorage", memoryStorage());
  root().remove("light", "dark", "high-contrast");
});
afterEach(() => {
  vi.unstubAllGlobals();
  root().remove("light", "dark", "high-contrast");
  window.history.replaceState(null, "", "/");
});

describe("the theme follows the account (w3/m531)", () => {
  it("opens in the browser's copy, then applies the account's stored theme over it", () => {
    localStorage.setItem(
      SETTINGS_STORAGE_KEY,
      JSON.stringify({ themeSetting: "LIGHT", logThemeSetting: "MATCH_APP" }),
    );
    renderProvider();
    expect(root().contains("light")).toBe(true);
    const { binding } = account({
      themeSetting: "DARK_HIGH_CONTRAST",
      logThemeSetting: "ALWAYS_LIGHT",
    });
    act(() => theme.bindAccount(binding));
    expect(root().contains("dark")).toBe(true);
    expect(root().contains("high-contrast")).toBe(true);
    expect(theme.theme).toBe("dark");
    expect(theme.highContrast).toBe(true);
    expect(theme.logTheme).toBe("ALWAYS_LIGHT");
    // The copy follows, so the pre-paint script paints it on the next load.
    expect(stored()).toEqual({
      themeSetting: "DARK_HIGH_CONTRAST",
      logThemeSetting: "ALWAYS_LIGHT",
    });
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark");
  });

  it("reads a browser that kept only the theme before the account did", () => {
    localStorage.setItem(THEME_STORAGE_KEY, "dark");
    renderProvider();
    expect(theme.theme).toBe("dark");
    expect(theme.highContrast).toBe(false);
    expect(theme.logTheme).toBe("MATCH_APP");
  });

  it("shows a change at once, writes it to the account, and keeps what the server returns", async () => {
    renderProvider();
    const { binding, writes } = account({
      themeSetting: "SYSTEM",
      logThemeSetting: "MATCH_APP",
    });
    act(() => theme.bindAccount(binding));
    act(() => void theme.setTheme("dark"));
    expect(root().contains("dark")).toBe(true);
    expect(writes.map((w) => w.patch)).toEqual([{ themeSetting: "DARK" }]);
    await act(async () => writes[0].resolve({ themeSetting: "DARK" }));
    expect(stored()?.themeSetting).toBe("DARK");

    act(() => void theme.setLogTheme("ALWAYS_DARK"));
    expect(writes[1].patch).toEqual({ logThemeSetting: "ALWAYS_DARK" });
  });

  it("writes nothing when the choice is what is shown already", () => {
    renderProvider();
    const { binding, writes } = account({
      themeSetting: "DARK",
      logThemeSetting: "ALWAYS_LIGHT",
    });
    act(() => theme.bindAccount(binding));
    act(() => void theme.setTheme("dark"));
    act(() => void theme.setLogTheme("ALWAYS_LIGHT"));
    act(() => void theme.setHighContrast(false));
    expect(writes).toEqual([]);
  });

  it("puts the previous theme back when the account refuses the write", async () => {
    renderProvider();
    const { binding, writes } = account({
      themeSetting: "LIGHT",
      logThemeSetting: "MATCH_APP",
    });
    act(() => theme.bindAccount(binding));
    expect(root().contains("light")).toBe(true);
    act(() => void theme.setTheme("dark"));
    expect(root().contains("dark")).toBe(true);
    await act(async () => writes[0].reject(new Error("refused")));
    expect(root().contains("light")).toBe(true);
    expect(theme.theme).toBe("light");
    expect(stored()?.themeSetting).toBe("LIGHT");
  });

  it("keeps High Contrast Mode across a theme change, and stores the twin", async () => {
    renderProvider();
    const { binding, writes } = account({
      themeSetting: "LIGHT",
      logThemeSetting: "MATCH_APP",
    });
    act(() => theme.bindAccount(binding));
    act(() => void theme.setHighContrast(true));
    expect(writes[0].patch).toEqual({ themeSetting: "LIGHT_HIGH_CONTRAST" });
    expect(root().contains("high-contrast")).toBe(true);
    await act(async () => writes[0].resolve(writes[0].patch));
    act(() => void theme.setTheme("system"));
    expect(writes[1].patch).toEqual({ themeSetting: "SYSTEM_HIGH_CONTRAST" });
    // Dashboard Theme still reads System, Light, Dark.
    expect(theme.theme).toBe("system");
    await act(async () => writes[1].resolve(writes[1].patch));
    act(() => void theme.setHighContrast(false));
    expect(writes[2].patch).toEqual({ themeSetting: "SYSTEM" });
    expect(root().contains("high-contrast")).toBe(false);
  });

  it("sends one write at a time, folding the changes made meanwhile into the next", async () => {
    renderProvider();
    const { binding, writes } = account({
      themeSetting: "LIGHT",
      logThemeSetting: "MATCH_APP",
    });
    act(() => theme.bindAccount(binding));
    act(() => void theme.setTheme("dark"));
    act(() => void theme.setHighContrast(true));
    act(() => void theme.setLogTheme("ALWAYS_LIGHT"));
    // Only the first is in flight; the page already shows all three.
    expect(writes.map((w) => w.patch)).toEqual([{ themeSetting: "DARK" }]);
    expect(theme.highContrast).toBe(true);
    // The account re-read while the write is in flight (the cache's older
    // answer, bound afresh as the shell binds it) does not undo a change
    // still being saved.
    act(() => theme.bindAccount({ ...binding }));
    expect(theme.theme).toBe("dark");
    expect(theme.logTheme).toBe("ALWAYS_LIGHT");
    await act(async () => writes[0].resolve({ themeSetting: "DARK" }));
    expect(writes.map((w) => w.patch)).toEqual([
      { themeSetting: "DARK" },
      { themeSetting: "DARK_HIGH_CONTRAST", logThemeSetting: "ALWAYS_LIGHT" },
    ]);
    await act(async () => writes[1].reject(new Error("refused")));
    // The refused fields go back to what the account last confirmed.
    expect(theme.highContrast).toBe(false);
    expect(theme.theme).toBe("dark");
    expect(theme.logTheme).toBe("MATCH_APP");
  });

  it("writes a change made before the account was bound once it is", () => {
    renderProvider();
    act(() => void theme.setTheme("dark"));
    const { binding, writes } = account({
      themeSetting: "LIGHT",
      logThemeSetting: "ALWAYS_LIGHT",
    });
    act(() => theme.bindAccount(binding));
    // The change wins over the account's older theme, and is saved.
    expect(theme.theme).toBe("dark");
    expect(theme.logTheme).toBe("ALWAYS_LIGHT");
    expect(writes.map((w) => w.patch)).toEqual([{ themeSetting: "DARK" }]);
  });

  it("resolves each change to whether the account kept it", async () => {
    renderProvider();
    const { binding, writes } = account({
      themeSetting: "LIGHT",
      logThemeSetting: "MATCH_APP",
    });
    act(() => theme.bindAccount(binding));
    let kept: Promise<boolean> = Promise.resolve(false);
    let refused: Promise<boolean> = Promise.resolve(true);
    act(() => {
      kept = theme.setTheme("dark");
    });
    // Folded into the next write while the first is in flight.
    act(() => {
      refused = theme.setLogTheme("ALWAYS_DARK");
    });
    await act(async () => writes[0].resolve(writes[0].patch));
    await expect(kept).resolves.toBe(true);
    await act(async () => writes[1].reject(new Error("refused")));
    await expect(refused).resolves.toBe(false);
    // A choice of what is saved already writes nothing and is kept.
    await expect(theme.setTheme("dark")).resolves.toBe(true);
    expect(writes).toHaveLength(2);
  });

  it("answers a change still waiting for the account as not saved when it signs out", async () => {
    renderProvider();
    let pending: Promise<boolean> = Promise.resolve(true);
    act(() => {
      pending = theme.setTheme("dark");
    });
    act(() => theme.bindAccount(null));
    await expect(pending).resolves.toBe(false);
  });

  it("previews a theme on the page without choosing, keeping or writing it (w3/097)", () => {
    renderProvider();
    const { binding, writes } = account({
      themeSetting: "LIGHT_HIGH_CONTRAST",
      logThemeSetting: "MATCH_APP",
    });
    act(() => theme.bindAccount(binding));
    act(() => theme.previewTheme("dark"));
    expect(root().contains("dark")).toBe(true);
    expect(root().contains("light")).toBe(false);
    // High Contrast Mode stays as saved.
    expect(root().contains("high-contrast")).toBe(true);
    // The chosen theme, the browser's copy and the account are untouched.
    expect(theme.theme).toBe("light");
    expect(stored()?.themeSetting).toBe("LIGHT_HIGH_CONTRAST");
    expect(writes).toEqual([]);
    act(() => theme.previewTheme(null));
    expect(root().contains("light")).toBe(true);
    expect(root().contains("dark")).toBe(false);
    // A theme chosen meanwhile replaces the preview.
    act(() => theme.previewTheme("dark"));
    act(() => void theme.setTheme("system"));
    expect(theme.theme).toBe("system");
    expect(root().contains("dark")).toBe(false);
    expect(writes.map((w) => w.patch)).toEqual([
      { themeSetting: "SYSTEM_HIGH_CONTRAST" },
    ]);
  });

  it("keeps a change in the browser when no account is bound (signed out)", () => {
    renderProvider();
    act(() => void theme.setTheme("dark"));
    expect(stored()?.themeSetting).toBe("DARK");
    act(() => theme.bindAccount(null));
    act(() => void theme.setTheme("light"));
    expect(stored()?.themeSetting).toBe("LIGHT");
  });

  it("follows a change another tab makes", () => {
    renderProvider();
    expect(theme.highContrast).toBe(false);
    const next = JSON.stringify({
      themeSetting: "DARK_HIGH_CONTRAST",
      logThemeSetting: "ALWAYS_LIGHT",
    });
    act(() => {
      window.dispatchEvent(
        new StorageEvent("storage", {
          key: SETTINGS_STORAGE_KEY,
          newValue: next,
        }),
      );
    });
    expect(root().contains("dark")).toBe(true);
    expect(root().contains("high-contrast")).toBe(true);
    expect(theme.logTheme).toBe("ALWAYS_LIGHT");
    // Another key, or a value that is not a copy, changes nothing.
    act(() => {
      window.dispatchEvent(
        new StorageEvent("storage", { key: "other", newValue: next }),
      );
      window.dispatchEvent(
        new StorageEvent("storage", {
          key: SETTINGS_STORAGE_KEY,
          newValue: "{}",
        }),
      );
    });
    expect(theme.theme).toBe("dark");
  });

  it("lets ?theme= win over the account's theme on this page", () => {
    window.history.replaceState(null, "", "/?theme=light");
    renderProvider();
    const { binding } = account({
      themeSetting: "DARK",
      logThemeSetting: "MATCH_APP",
    });
    act(() => theme.bindAccount(binding));
    expect(root().contains("light")).toBe(true);
    expect(theme.theme).toBe("light");
  });

  it("leaves the pre-paint script's class alone until the stored theme is read", () => {
    root().add("dark");
    localStorage.setItem(
      SETTINGS_STORAGE_KEY,
      JSON.stringify({ themeSetting: "DARK", logThemeSetting: "MATCH_APP" }),
    );
    // Every class <html> is given while the provider mounts, as it is given.
    const added: string[] = [];
    const add = vi
      .spyOn(DOMTokenList.prototype, "add")
      .mockImplementation(function (this: DOMTokenList, ...names: string[]) {
        if (this === root()) added.push(...names);
        names.forEach((name) =>
          DOMTokenList.prototype.toggle.call(this, name, true),
        );
      });
    try {
      renderProvider();
    } finally {
      add.mockRestore();
    }
    // Never a light frame between the script's dark and the provider's.
    expect(added).not.toContain("light");
    expect(root().contains("dark")).toBe(true);
  });
});
