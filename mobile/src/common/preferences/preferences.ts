// Persisted device-local UI preferences: theme mode (light/dark/system) and
// interface language. Neither is a secret — the values only tune this device's
// appearance — so plain AsyncStorage is fine (never the reviewed auth store).
// The pure parse/resolve helpers are unit-tested; the storage wrappers are thin,
// best-effort I/O that fall back to the caller's default on any failure.
import AsyncStorage from "@react-native-async-storage/async-storage";

export type ThemeMode = "light" | "dark" | "system";

// The bex.co and dashboard locale set, in the same order (the dashboard's
// src/i18n/config.ts), so web and mobile offer the same languages.
export const SUPPORTED_LANGUAGES = [
  "en",
  "zh",
  "ko",
  "ja",
  "es",
  "pt",
  "ru",
  "de",
] as const;
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

const THEME_MODE_KEY = "bex.mobile.theme-mode.v1";
const LANGUAGE_KEY = "bex.mobile.language.v1";

const THEME_MODES: readonly ThemeMode[] = ["light", "dark", "system"];

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
    (SUPPORTED_LANGUAGES as readonly string[]).includes(value)
    ? (value as SupportedLanguage)
    : null;
}

// One of the device's preferred locales, as expo-localization's getLocales()
// reports it.
export type DeviceLocale = {
  languageCode: string | null;
  languageScriptCode?: string | null;
  regionCode?: string | null;
};

// Chinese read in Traditional script: an explicit Hant script, or (Android
// often reports no script) a Traditional-script region.
const TRADITIONAL_CHINESE_REGIONS = ["TW", "HK", "MO"];
function isTraditionalChinese(locale: DeviceLocale): boolean {
  return (
    locale.languageScriptCode === "Hant" ||
    TRADITIONAL_CHINESE_REGIONS.includes(locale.regionCode ?? "")
  );
}

// Pure: the first device locale the app speaks, else null so the caller keeps
// English. The zh catalog is Simplified Chinese, so a Traditional-script
// locale (zh-Hant, zh-TW, zh-HK, zh-MO) is skipped in favour of the next preference.
export function resolveDeviceLanguage(
  locales: readonly DeviceLocale[],
): SupportedLanguage | null {
  for (const locale of locales) {
    const language = parseLanguage(locale.languageCode?.toLowerCase());
    if (language === "zh" && isTraditionalChinese(locale)) continue;
    if (language) return language;
  }
  return null;
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
