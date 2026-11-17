import { beforeEach, describe, expect, it, vi } from "vitest";

const { getRequest } = vi.hoisted(() => ({ getRequest: vi.fn() }));
vi.mock("@tanstack/react-start/server", () => ({
  getRequest: () => getRequest(),
}));

// The global test setup loads the real server entry before the mock above takes
// effect, so re-import fresh (as search-params' server test does).
async function importGetReferrerOnServer() {
  vi.resetModules();
  return (await import("../server")).getReferrerOnServer;
}

// A Deploy button clicked on another site is a full page load, so /deploy's
// referrer fallback (w33/003) runs on the server and reads the header.
describe("getReferrerOnServer", () => {
  beforeEach(() => getRequest.mockReset());

  it("reads the request's Referer header", async () => {
    getRequest.mockReturnValue(
      new Request("https://dashboard.bex.co/deploy", {
        headers: { referer: "https://gitlab.com/a/b" },
      }),
    );
    expect((await importGetReferrerOnServer())()).toBe(
      "https://gitlab.com/a/b",
    );
  });

  it("is empty when the browser sent none", async () => {
    getRequest.mockReturnValue(new Request("https://dashboard.bex.co/deploy"));
    expect((await importGetReferrerOnServer())()).toBe("");
  });
});
