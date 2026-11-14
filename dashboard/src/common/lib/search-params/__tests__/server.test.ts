import { beforeEach, describe, expect, it, vi } from "vitest";

const { getRequest } = vi.hoisted(() => ({ getRequest: vi.fn() }));
vi.mock("@tanstack/react-start/server", () => ({
  getRequest: () => getRequest(),
}));

// The global test setup imports modules that load the real server entry before
// the mock above takes effect, so re-import fresh (as detect-language's test does).
async function importGetSearchParamOnServer() {
  vi.resetModules();
  return (await import("../server")).getSearchParamOnServer;
}

function requestFor(search: string) {
  getRequest.mockReturnValue(new Request(`https://dashboard.bex.co/${search}`));
}

// R25-15 (w21/m394 t017): URLSearchParams.get already percent-decodes, so a
// second decodeURIComponent threw URIError on `?lang=%25` or `?lang=%` and the
// server answered 500 on any route.
describe("getSearchParamOnServer", () => {
  beforeEach(() => getRequest.mockReset());

  it("returns the decoded value once", async () => {
    requestFor("?lang=zh-CN");
    expect((await importGetSearchParamOnServer())("lang")).toBe("zh-CN");
  });

  it.each([
    ["?lang=%25", "lang", "%"],
    ["?lang=%", "lang", "%"],
    ["?locale=%25zz", "locale", "%zz"],
    ["?lang=%E4%B8%AD", "lang", "中"],
    ["?lang=%2541", "lang", "%41"],
  ])("does not throw on %s", async (search, key, want) => {
    requestFor(search);
    expect((await importGetSearchParamOnServer())(key)).toBe(want);
  });

  it("returns null for a missing parameter", async () => {
    requestFor("?other=1");
    expect((await importGetSearchParamOnServer())("lang")).toBeNull();
  });
});
