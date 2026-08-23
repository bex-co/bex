import { parseLanguage, parseThemeMode, resolveScheme } from "../preferences";

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
    expect(parseLanguage("en")).toBe("en");
    expect(parseLanguage("zh")).toBe("zh");
    expect(parseLanguage("fr")).toBe(null);
    expect(parseLanguage(undefined)).toBe(null);
  });

  it("follows the system scheme only when the mode is system", () => {
    expect(resolveScheme("system", "dark")).toBe("dark");
    expect(resolveScheme("system", "light")).toBe("light");
    // An explicit mode wins regardless of the OS setting.
    expect(resolveScheme("light", "dark")).toBe("light");
    expect(resolveScheme("dark", "light")).toBe("dark");
  });
});
