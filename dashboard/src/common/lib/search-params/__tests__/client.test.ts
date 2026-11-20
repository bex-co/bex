import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { getSearchParamOnClient } from "../client";
import { useLanguageHydrationSync } from "@/i18n/use-language-hydration-sync";
import { memoryStorage } from "@/test/mocks/storage";

function visit(search: string) {
  window.history.replaceState(null, "", `/services${search}`);
}

afterEach(() => {
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});

// URLSearchParams.get already percent-decodes. A second decodeURIComponent
// threw URIError on `?lang=%25` or `?lang=%` inside the root's post-hydration
// effect, and the error page replaced the whole app. The server copy got the
// same fix in R25-15 (see server.test.ts).
describe("getSearchParamOnClient", () => {
  it("returns the decoded value once", () => {
    visit("?lang=zh-CN");
    expect(getSearchParamOnClient("lang")).toBe("zh-CN");
  });

  it.each([
    ["?lang=%25", "lang", "%"],
    ["?lang=%", "lang", "%"],
    ["?locale=%25zz", "locale", "%zz"],
    ["?lang=%E4%B8%AD", "lang", "中"],
    ["?lang=%2541", "lang", "%41"],
    ["?lang=zh", "lang", "zh"],
  ])("does not throw on %s", (search, key, want) => {
    visit(search);
    expect(getSearchParamOnClient(key)).toBe(want);
  });

  it("returns null for a missing parameter", () => {
    visit("?other=1");
    expect(getSearchParamOnClient("lang")).toBeNull();
  });
});

describe("the post-hydration language sync on a malformed param", () => {
  it.each(["?lang=%25", "?locale=%25zz", "?lang=%257A%2568"])(
    "neither throws nor persists a language on %s",
    (search) => {
      const storage = memoryStorage();
      vi.stubGlobal("localStorage", storage);
      visit(search);
      expect(() => renderHook(() => useLanguageHydrationSync())).not.toThrow();
      // `%257A%2568` is "%7A%68" after one decode: no language, as on the
      // server. A second decode turned it into "zh" and persisted it.
      expect(storage.getItem("i18nextLng")).toBeNull();
    },
  );
});
