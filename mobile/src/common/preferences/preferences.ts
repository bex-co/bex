// Persisted device-local UI preferences: theme mode (light/dark/system) and
// interface language. Neither is a secret — the values only tune this device's
// appearance — so plain AsyncStorage is fine (never the reviewed auth store).
// The pure parse/resolve helpers are unit-tested; the storage wrappers are thin,
// best-effort I/O that fall back to the caller's default on any failure.
import AsyncStorage from "@react-native-async-storage/async-storage";

export type ThemeMode = "light" | "dark" | "system";
export type SupportedLanguage = "en" | "zh";

const THEME_MODE_KEY = "bex.mobile.theme-mode.v1";
const LANGUAGE_KEY = "bex.mobile.language.v1";

const THEME_MODES: readonly ThemeMode[] = ["light", "dark", "system"];
const LANGUAGES: readonly SupportedLanguage[] = ["en", "zh"];

// Pure: narrow an untrusted stored value to a known theme mode, else null so
// the caller keeps its default instead of trusting corrupt storage.
export function parseThemeMode(value: unknown): ThemeMode | null {
  return typeof value === "string" && THEME_MODES.includes(value as ThemeMode)
    ? (value as ThemeMode)
    : null;
}

// Pure: narrow an untrusted stored value to a supported language, else null.
export function parseLanguage(value: unknown): SupportedLanguage | null {
  return typeof value === "string" &&
    LANGUAGES.includes(value as SupportedLanguage)
    ? (value as SupportedLanguage)
    : null;
}

// Pure: the concrete scheme to render — "system" tracks the OS setting, an
// explicit mode overrides it.
export function resolveScheme(
  mode: ThemeMode,
  systemScheme: "light" | "dark",
): "light" | "dark" {
  return mode === "system" ? systemScheme : mode;
}

export async function loadThemeMode(): Promise<ThemeMode | null> {
  const raw = await AsyncStorage.getItem(THEME_MODE_KEY).catch(() => null);
  return parseThemeMode(raw);
}

export async function saveThemeMode(mode: ThemeMode): Promise<void> {
  await AsyncStorage.setItem(THEME_MODE_KEY, mode).catch(() => undefined);
}

export async function loadLanguage(): Promise<SupportedLanguage | null> {
  const raw = await AsyncStorage.getItem(LANGUAGE_KEY).catch(() => null);
  return parseLanguage(raw);
}

export async function saveLanguage(language: SupportedLanguage): Promise<void> {
  await AsyncStorage.setItem(LANGUAGE_KEY, language).catch(() => undefined);
}
