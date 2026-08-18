import { resolveSupportedLanguage } from "../language";

describe("resolveSupportedLanguage", () => {
  it("selects Chinese for the device's Chinese language code", () => {
    expect(resolveSupportedLanguage("zh")).toBe("zh");
    expect(resolveSupportedLanguage("ZH")).toBe("zh");
  });

  it("falls back to English for every unsupported or missing language", () => {
    expect(resolveSupportedLanguage("en")).toBe("en");
    expect(resolveSupportedLanguage("de")).toBe("en");
    expect(resolveSupportedLanguage(null)).toBe("en");
    expect(resolveSupportedLanguage(undefined)).toBe("en");
  });
});
