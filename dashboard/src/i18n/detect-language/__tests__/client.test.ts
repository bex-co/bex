import { describe, it, expect, vi, beforeEach } from "vitest";

const mockGetSearchParamOnClient = vi.fn();
const mockGetCookie = vi.fn();

vi.mock("@/common/lib/search-params/client", () => ({
  getSearchParamOnClient: (key: string) => mockGetSearchParamOnClient(key),
}));
vi.mock("@/common/hooks/use-cookie-storage-state/cookie", () => ({
  getCookie: (key: string) => mockGetCookie(key),
}));

// The global test setup imports "@/i18n/init", which eagerly loads this
// module's real dependencies before the mocks above take effect. Reset the
// module registry and re-import fresh in each test so the mocks apply.
async function importDetectLanguageOnClient() {
  vi.resetModules();
  return (await import("../client")).detectLanguageOnClient;
}

describe("detectLanguageOnClient", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetSearchParamOnClient.mockReturnValue(null);
    mockGetCookie.mockReturnValue(undefined);
  });

  it("falls back to the default language when nothing is set", async () => {
    const detectLanguageOnClient = await importDetectLanguageOnClient();
    expect(detectLanguageOnClient()).toBe("en");
  });

  it("prefers the ?lang= URL param over the cookie", async () => {
    mockGetSearchParamOnClient.mockImplementation((key) => (key === "lang" ? "zh" : null));
    mockGetCookie.mockReturnValue("en");

    const detectLanguageOnClient = await importDetectLanguageOnClient();
    expect(detectLanguageOnClient()).toBe("zh");
  });

  it("accepts ?locale= as a fallback for ?lang=", async () => {
    mockGetSearchParamOnClient.mockImplementation((key) => (key === "locale" ? "zh" : null));

    const detectLanguageOnClient = await importDetectLanguageOnClient();
    expect(detectLanguageOnClient()).toBe("zh");
  });

  it("ignores an unsupported URL language and falls through to the cookie", async () => {
    mockGetSearchParamOnClient.mockImplementation((key) => (key === "lang" ? "fr" : null));
    mockGetCookie.mockReturnValue("zh");

    const detectLanguageOnClient = await importDetectLanguageOnClient();
    expect(detectLanguageOnClient()).toBe("zh");
  });

  it("falls back to the cookie when no URL param is set", async () => {
    mockGetCookie.mockReturnValue("zh");

    const detectLanguageOnClient = await importDetectLanguageOnClient();
    expect(detectLanguageOnClient()).toBe("zh");
  });

  it("ignores an unsupported cookie value", async () => {
    mockGetCookie.mockReturnValue("fr");

    const detectLanguageOnClient = await importDetectLanguageOnClient();
    expect(detectLanguageOnClient()).toBe("en");
  });
});
