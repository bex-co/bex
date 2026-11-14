import { describe, expect, it } from "vitest";

import assetsMiss from "../middleware/assets-miss";
import htmlNeverPublic from "../plugins/html-never-public";

const middlewareEvent = (path: string) => ({
  url: new URL(`https://dashboard.bex.co${path}`),
});

// The Nitro "response" hook the plugin registers, captured through a fake
// nitroApp so the test runs the plugin's own wiring.
function applyHook(
  path: string,
  body: string,
  init: ResponseInit & { headers: Record<string, string> },
): Response {
  let hook: ((res: Response, event: { req: Request }) => void) | undefined;
  htmlNeverPublic({
    hooks: {
      hook: (name: string, fn: typeof hook) => {
        if (name === "response") hook = fn;
      },
    },
  } as never);
  if (!hook) throw new Error("the plugin registered no response hook");
  const res = new Response(body, init);
  hook(res, { req: new Request(`https://dashboard.bex.co${path}`) });
  return res;
}

describe("the /assets/** miss handler", () => {
  it.each([
    "/assets/qa-missing.css",
    "/assets/nope/qa.js",
    "/assets/",
    "/assets",
  ])("answers %s with 404, no-store and no page body", async (path) => {
    const res = assetsMiss(middlewareEvent(path))!;
    expect(res.status).toBe(404);
    expect(res.headers.get("cache-control")).toBe("no-store");
    expect(res.headers.get("content-type")).not.toMatch(/html/);
    expect(res.headers.get("x-content-type-options")).toBe("nosniff");
    expect(await res.text()).not.toMatch(/<html|<script/i);
  });

  it.each(["/services", "/assetsx/a.css"])(
    "leaves %s to the renderer",
    (path) => {
      expect(assetsMiss(middlewareEvent(path))).toBeUndefined();
    },
  );
});

describe("the response hook", () => {
  const immutable = "public, max-age=31536000, immutable";

  it.each([
    ["a missing /assets file's page", "/assets/missing.css", immutable],
    ["any page", "/services/srv-1", "max-age=300, Public"],
  ])(
    "rewrites a public text/html response (%s) to private, no-store",
    (_, path, cc) => {
      const res = applyHook(path, "<html>session</html>", {
        headers: {
          "content-type": "text/html; charset=utf-8",
          "cache-control": cc,
        },
      });
      expect(res.headers.get("cache-control")).toBe("private, no-store");
    },
  );

  it("keeps a page's own private, no-cache", () => {
    const res = applyHook("/services", "<html></html>", {
      headers: {
        "content-type": "text/html",
        "cache-control": "private, no-cache",
      },
    });
    expect(res.headers.get("cache-control")).toBe("private, no-cache");
  });

  it("forces no-store on a missing /assets/** file, whatever the route rule set", () => {
    const res = applyHook("/assets/missing.js", "Not found", {
      status: 404,
      headers: { "content-type": "text/plain", "cache-control": immutable },
    });
    expect(res.headers.get("cache-control")).toBe("no-store");
  });

  it.each([
    [
      "a real build file",
      "/assets/index-abc123.js",
      "text/javascript; charset=utf-8",
      immutable,
    ],
    [
      "an association document",
      "/.well-known/assetlinks.json",
      "application/json; charset=utf-8",
      "public, max-age=300",
    ],
  ])("leaves %s's public header alone", (_, path, type, cc) => {
    const res = applyHook(path, "x", {
      headers: { "content-type": type, "cache-control": cc },
    });
    expect(res.headers.get("cache-control")).toBe(cc);
  });
});
