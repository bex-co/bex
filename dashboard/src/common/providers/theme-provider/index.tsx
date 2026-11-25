import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ThemeProviderContext } from "./context.ts";
import type { AccountThemeBinding, Theme } from "./type.ts";
import {
  baseTheme,
  DEFAULT_THEME_SETTINGS,
  HIGH_CONTRAST_CLASS,
  isHighContrast,
  isTheme,
  parseStoredSettings,
  readBrowserSettings,
  SETTINGS_STORAGE_KEY,
  toThemeSetting,
  writeBrowserSettings,
  type LogThemeSetting,
  type ThemeSettings,
} from "./settings.ts";
import { applyThemeColor } from "./theme-color.ts";
import { getSystemTheme, SYSTEM_DARK_QUERY } from "./utils.ts";

/** `?theme=light|dark|system` wins over the stored theme, for this page. */
const getThemeFromQuery = (): Theme | null => {
  const value = new URLSearchParams(window.location.search).get("theme");
  return isTheme(value) ? value : null;
};

const same = (a: ThemeSettings, b: ThemeSettings) =>
  a.themeSetting === b.themeSetting && a.logThemeSetting === b.logThemeSetting;

/** `source`'s value for each of `keys`. */
const pick = (source: ThemeSettings, keys: string[]): Partial<ThemeSettings> =>
  Object.fromEntries(
    keys.map((key) => [key, source[key as keyof ThemeSettings]]),
  );

/**
 * The dashboard's theme (w3/m531). The settings are the account's, in
 * Render's enums: Dashboard Theme with High Contrast Mode as its
 * `_HIGH_CONTRAST` twin, and Log Explorer Theme. The browser keeps a copy
 * (`SETTINGS_STORAGE_KEY`), which the pre-paint script applies before the
 * first paint and other tabs follow; the signed-in shell binds the account
 * (`bindAccount`), whose stored settings then win, and every change is
 * written to it. A change shows at once; a write the server refuses puts the
 * previous setting back. Writes go one at a time, later changes folded into
 * the next, so two answers can never land out of order; each change resolves
 * to whether the account kept it. A preview (`previewTheme`: Appearance's
 * Dashboard Theme row being edited, w3/097) shows a theme on the page and is
 * written neither to the browser's copy nor to the account. The theme shown
 * also gives `<meta name="theme-color">` its page colour (w3/098), as
 * Render's provider does.
 */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  // Read after hydration, so the server's render and the first client render
  // agree. Until then <html> keeps the class the pre-paint script gave it.
  const [settings, setSettings] = useState<ThemeSettings>(
    DEFAULT_THEME_SETTINGS,
  );
  const [ready, setReady] = useState(false);
  const [override, setOverride] = useState<Theme | null>(null);
  const [preview, setPreview] = useState<Theme | null>(null);
  // The latest settings, for a write that settles after newer changes.
  const latest = useRef(settings);
  // The bound account, with its settings as the server last answered them:
  // what a refused write puts back.
  const account = useRef<{
    binding: AccountThemeBinding;
    confirmed: ThemeSettings;
  } | null>(null);
  // Changes not yet sent (all of them, before the account is bound) with
  // whoever waits to hear whether they were saved, and the one write in
  // flight.
  const queued = useRef<Partial<ThemeSettings>>({});
  const waiting = useRef<((saved: boolean) => void)[]>([]);
  const sending = useRef<Partial<ThemeSettings> | null>(null);

  /** Shows `next`; false when it is what is shown already. */
  const show = useCallback((next: ThemeSettings) => {
    if (same(next, latest.current)) return false;
    latest.current = next;
    setSettings(next);
    return true;
  }, []);
  const keep = useCallback(
    (next: ThemeSettings) => {
      if (show(next)) writeBrowserSettings(next);
    },
    [show],
  );

  useEffect(() => {
    // The stored theme is read after hydration.
    show(readBrowserSettings());
    // eslint-disable-next-line react-hooks/set-state-in-effect -- read after hydration, as the stored theme is
    setOverride(getThemeFromQuery());
    setReady(true);
  }, [show]);

  // Another tab's change (Render's provider listens the same way).
  useEffect(() => {
    const follow = (event: StorageEvent) => {
      if (event.key !== SETTINGS_STORAGE_KEY) return;
      const next = parseStoredSettings(event.newValue);
      if (next) show(next);
    };
    window.addEventListener("storage", follow);
    return () => window.removeEventListener("storage", follow);
  }, [show]);

  const base = override ?? baseTheme(settings.themeSetting);
  const shown = preview ?? base;
  const highContrast = isHighContrast(settings.themeSetting);

  useEffect(() => {
    if (!ready) return;
    const apply = () => {
      const root = window.document.documentElement;
      const target = shown === "system" ? getSystemTheme() : shown;
      if (!root.classList.contains(target)) {
        root.classList.remove("light", "dark");
        root.classList.add(target);
      }
      root.classList.toggle(HIGH_CONTRAST_CLASS, highContrast);
      applyThemeColor(target);
    };
    apply();
    // "System" follows the preference while the page is open, not only the
    // value it had when System was chosen (w4/315).
    if (shown !== "system") return;
    const query = window.matchMedia(SYSTEM_DARK_QUERY);
    query.addEventListener("change", apply);
    return () => query.removeEventListener("change", apply);
  }, [shown, highContrast, ready]);

  /** Sends the queued changes, one write at a time; a call while one is in
   *  flight leaves the rest to that loop. A refused write puts back each field
   *  it carried that no newer change is waiting to replace. */
  const flush = useCallback(async () => {
    if (sending.current) return;
    for (;;) {
      const bound = account.current;
      const patch = queued.current;
      if (!bound || Object.keys(patch).length === 0) return;
      const waiters = waiting.current;
      queued.current = {};
      waiting.current = [];
      sending.current = patch;
      let saved = false;
      try {
        const kept = await bound.binding.write(patch);
        // The account may have been bound again meanwhile (its cache read
        // after a save); what it confirms is the one now bound.
        if (account.current) {
          account.current.confirmed = {
            ...account.current.confirmed,
            ...kept,
          };
        }
        saved = true;
      } catch {
        const now = account.current;
        if (now) {
          const replacedSince = (key: string) => key in queued.current;
          keep({
            ...latest.current,
            ...pick(
              now.confirmed,
              Object.keys(patch).filter((key) => !replacedSince(key)),
            ),
          });
        }
      } finally {
        sending.current = null;
      }
      for (const settle of waiters) settle(saved);
    }
  }, [keep]);

  const change = useCallback(
    (patch: Partial<ThemeSettings>) => {
      // A choice of what is shown already changes nothing and writes nothing.
      const changed = Object.fromEntries(
        Object.entries(patch).filter(
          ([key, value]) =>
            latest.current[key as keyof ThemeSettings] !== value,
        ),
      );
      if (Object.keys(changed).length === 0) return Promise.resolve(true);
      keep({ ...latest.current, ...changed });
      queued.current = { ...queued.current, ...changed };
      const saved = new Promise<boolean>((settle) => {
        waiting.current.push(settle);
      });
      void flush();
      return saved;
    },
    [keep, flush],
  );

  const bindAccount = useCallback(
    (binding: AccountThemeBinding | null) => {
      account.current = binding && { binding, confirmed: binding.settings };
      if (!binding) {
        // Signed out: the changes not yet sent never will be.
        for (const settle of waiting.current) settle(false);
        queued.current = {};
        waiting.current = [];
        return;
      }
      // The account's settings win, and the browser's copy takes them, but a
      // change not yet saved keeps its value until its write settles.
      keep({
        ...binding.settings,
        ...pick(
          latest.current,
          Object.keys({ ...sending.current, ...queued.current }),
        ),
      });
      void flush();
    },
    [keep, flush],
  );

  const setTheme = useCallback(
    (theme: Theme) => {
      setOverride(null);
      setPreview(null);
      return change({
        themeSetting: toThemeSetting(
          theme,
          isHighContrast(latest.current.themeSetting),
        ),
      });
    },
    [change],
  );
  const setHighContrast = useCallback(
    (on: boolean) =>
      change({
        themeSetting: toThemeSetting(
          baseTheme(latest.current.themeSetting),
          on,
        ),
      }),
    [change],
  );
  const setLogTheme = useCallback(
    (logThemeSetting: LogThemeSetting) => change({ logThemeSetting }),
    [change],
  );

  const value = useMemo(
    () => ({
      theme: base,
      setTheme,
      previewTheme: setPreview,
      highContrast,
      setHighContrast,
      logTheme: settings.logThemeSetting,
      setLogTheme,
      ready,
      bindAccount,
    }),
    [
      base,
      setTheme,
      highContrast,
      setHighContrast,
      settings.logThemeSetting,
      setLogTheme,
      ready,
      bindAccount,
    ],
  );

  return (
    <ThemeProviderContext.Provider value={value}>
      {children}
    </ThemeProviderContext.Provider>
  );
}

/* eslint-disable-next-line react-refresh/only-export-components */
export { useTheme, useIsDarkTheme } from "./hooks.ts";
