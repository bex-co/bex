import { isAllowedHttpsUrl, safeDeepLink } from "../url-utils";

describe("URL utilities", () => {
  it("allows only known in-app roots", () => {
    expect(safeDeepLink("/status/srv-123")).toBe("/status/srv-123");
    expect(safeDeepLink("settings/secrets")).toBe(null);
  });

  it("requires HTTPS and an exact host", () => {
    expect(
      isAllowedHttpsUrl("https://github.com/bex-co/bex", ["github.com"]),
    ).toBe(true);
    expect(
      isAllowedHttpsUrl("http://github.com/bex-co/bex", ["github.com"]),
    ).toBe(false);
    expect(
      isAllowedHttpsUrl("https://github.com.evil.test/x", ["github.com"]),
    ).toBe(false);
  });
});
