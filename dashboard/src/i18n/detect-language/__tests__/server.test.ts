import { describe, it, expect, vi, beforeEach } from "vitest";

const mockGetSearchParamOnServer = vi.fn();
const mockGetCookie = vi.fn();
const mockGetRequestHeader = vi.fn();

vi.mock("@/common/lib/search-params/server", () => ({
  getSearchParamOnServer: (key: string) => mockGetSearchParamOnServer(key),
}));
vi.mock("@/common/hooks/use-cookie-storage-state/cookie", () => ({
  getCookie: (key: string) => mockGetCookie(key),
}));
vi.mock("@tanstack/react-start/server", () => ({
  getRequestHeader: (key: string) => mockGetRequestHeader(key),
}));

// The global test setup imports "@/i18n/init", which eagerly loads this
// module's real dependencies before the mocks above take effect. Reset the
// module registry and re-import fresh in each test so the mocks apply.
async function importDetectLanguageOnServer() {
  vi.resetModules();
  return (await import("../server")).detectLanguageOnServer;
}

describe("detectLanguageOnServer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetSearchParamOnServer.mockReturnValue(null);
    mockGetCookie.mockReturnValue(undefined);
    mockGetRequestHeader.mockReturnValue(undefined);
  });

  it("falls back to the default language when nothing is set", async () => {
    const detectLanguageOnServer = await importDetectLanguageOnServer();
    expect(detectLanguageOnServer()).toBe("en");
  });

  it("prefers the ?lang= URL param over everything else", async () => {
    mockGetSearchParamOnServer.mockImplementation((key) =>
      key === "lang" ? "zh" : null,
    );
    mockGetCookie.mockReturnValue("en");
    mockGetRequestHeader.mockReturnValue("en");

    const detectLanguageOnServer = await importDetectLanguageOnServer();
    expect(detectLanguageOnServer()).toBe("zh");
  });

  it("accepts ?locale= as a fallback for ?lang=", async () => {
    mockGetSearchParamOnServer.mockImplementation((key) =>
      key === "locale" ? "zh" : null,
    );

    const detectLanguageOnServer = await importDetectLanguageOnServer();
    expect(detectLanguageOnServer()).toBe("zh");
  });

  it("ignores an unsupported URL language and falls through to the cookie", async () => {
    mockGetSearchParamOnServer.mockImplementation((key) =>
      key === "lang" ? "fr" : null,
    );
    mockGetCookie.mockReturnValue("zh");

    const detectLanguageOnServer = await importDetectLanguageOnServer();
    expect(detectLanguageOnServer()).toBe("zh");
  });

  it("falls back to the cookie when no URL param is set", async () => {
    mockGetCookie.mockReturnValue("zh");
    mockGetRequestHeader.mockReturnValue("en");

    const detectLanguageOnServer = await importDetectLanguageOnServer();
    expect(detectLanguageOnServer()).toBe("zh");
  });

  it("falls back to Accept-Language when there's no URL param or cookie", async () => {
    mockGetRequestHeader.mockReturnValue("fr-FR,fr;q=0.9,zh;q=0.8,en;q=0.7");

    const detectLanguageOnServer = await importDetectLanguageOnServer();
    expect(detectLanguageOnServer()).toBe("zh");
  });

  it("matches Accept-Language by primary subtag, ignoring region", async () => {
    mockGetRequestHeader.mockReturnValue("zh-CN,zh;q=0.9");

    const detectLanguageOnServer = await importDetectLanguageOnServer();
    expect(detectLanguageOnServer()).toBe("zh");
  });
});
