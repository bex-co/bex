import {
  parseLanguage,
  parseThemeMode,
  resolveDeviceLanguage,
  resolveScheme,
} from "../preferences";

describe("preferences", () => {
  it("accepts only known theme modes", () => {
    expect(parseThemeMode("light")).toBe("light");
    expect(parseThemeMode("dark")).toBe("dark");
    expect(parseThemeMode("system")).toBe("system");
    expect(parseThemeMode("neon")).toBe(null);
    expect(parseThemeMode(null)).toBe(null);
    expect(parseThemeMode(42)).toBe(null);
  });

  it("accepts only supported languages", () => {
    for (const language of [
      "en",
      "zh",
      "ko",
      "ja",
      "es",
      "pt",
      "ru",
      "de",
    ] as const) {
      expect(parseLanguage(language)).toBe(language);
    }
    expect(parseLanguage("fr")).toBe(null);
    expect(parseLanguage("zh-CN")).toBe(null);
    expect(parseLanguage(undefined)).toBe(null);
  });

  it("follows the first device locale the app speaks", () => {
    expect(resolveDeviceLanguage([{ languageCode: "de" }])).toBe("de");
    expect(
      resolveDeviceLanguage([
        { languageCode: "fr" },
        { languageCode: "pt", regionCode: "PT" },
      ]),
    ).toBe("pt");
    expect(
      resolveDeviceLanguage([
        { languageCode: "zh", languageScriptCode: "Hans" },
      ]),
    ).toBe("zh");
    expect(resolveDeviceLanguage([{ languageCode: "fr" }])).toBe(null);
    expect(resolveDeviceLanguage([{ languageCode: null }])).toBe(null);
    expect(resolveDeviceLanguage([])).toBe(null);
  });

  it("never reads Traditional Chinese as the Simplified zh catalog", () => {
    expect(
      resolveDeviceLanguage([
        { languageCode: "zh", languageScriptCode: "Hant", regionCode: "TW" },
        { languageCode: "ja" },
      ]),
    ).toBe("ja");
    // Android often reports no script for zh-HK / zh-TW.
    expect(
      resolveDeviceLanguage([{ languageCode: "zh", regionCode: "HK" }]),
    ).toBe(null);
  });

  it("follows the system scheme only when the mode is system", () => {
    expect(resolveScheme("system", "dark")).toBe("dark");
    expect(resolveScheme("system", "light")).toBe("light");
    // An explicit mode wins regardless of the OS setting.
    expect(resolveScheme("light", "dark")).toBe("light");
    expect(resolveScheme("dark", "light")).toBe("dark");
  });
});
