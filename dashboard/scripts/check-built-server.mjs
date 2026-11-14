// The serving contract of the production build (w4/m348), checked against the
// built server itself rather than a header map: run `yarn build`, then
// `yarn test:built-server`. CI runs it after its production build step.
//
// A miss under an asset directory must answer 404 `no-store` and never the
// HTML shell, or Cloudflare and the browser keep HTML under a script URL for a
// year and every page importing that chunk fails until site data is cleared.

import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { readdirSync } from "node:fs";
import { createServer } from "node:net";
import { after, before, test } from "node:test";
import { fileURLToPath } from "node:url";

const OUTPUT = fileURLToPath(new URL("../.output/", import.meta.url));
// `ASSETS_DIR` in server/assets-paths.ts; a drift fails `builtFile`.
const ASSETS = "_assets";
const IMMUTABLE = "public, max-age=31536000, immutable";
const SECURITY_HEADERS = [
  "content-security-policy",
  "strict-transport-security",
  "x-content-type-options",
  "x-frame-options",
];

function freePort() {
  return new Promise((resolve, reject) => {
    const probe = createServer().listen(0, () => {
      const { port } = probe.address();
      probe.close(() => resolve(port));
    });
    probe.on("error", reject);
  });
}

let server;
let origin;

before(async () => {
  const port = await freePort();
  origin = `http://127.0.0.1:${port}`;
  server = spawn("node", [`${OUTPUT}server/index.mjs`], {
    env: { ...process.env, NODE_ENV: "production", PORT: String(port) },
    stdio: ["ignore", "ignore", "inherit"],
  });
  for (let i = 0; i < 100; i++) {
    try {
      await fetch(`${origin}/health`);
      return;
    } catch {
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
  }
  throw new Error("the built server did not start");
});

after(() => server?.kill());

// The miss middleware answers with its own Response, which carries only
// `nosniff` of the route rules' security headers; a text/plain 404 needs no
// CSP or frame policy.
async function head(path, required = SECURITY_HEADERS) {
  // HEAD, as `curl -I` and a CDN revalidation send it; GET answers the same.
  const res = await fetch(origin + path, {
    method: "HEAD",
    redirect: "manual",
  });
  for (const name of required) {
    assert.ok(res.headers.get(name), `${path} lacks ${name}`);
  }
  return res;
}

function builtFile(extension) {
  const name = readdirSync(`${OUTPUT}public/${ASSETS}`).find((file) =>
    file.endsWith(extension),
  );
  assert.ok(name, `the build has no ${extension} asset`);
  return `/${ASSETS}/${name}`;
}

test("a built chunk and stylesheet are immutable", async () => {
  for (const [path, type] of [
    [builtFile(".js"), "text/javascript"],
    [builtFile(".css"), "text/css"],
  ]) {
    const res = await head(path);
    assert.equal(res.status, 200, path);
    assert.match(res.headers.get("content-type"), new RegExp(`^${type}`));
    assert.equal(res.headers.get("cache-control"), IMMUTABLE, path);
  }
});

test("a missing asset answers 404 no-store, never the HTML shell", async () => {
  const chunk = builtFile(".js").split("/").pop();
  for (const path of [
    `/${ASSETS}/qa-missing-chunk-Xy12Ab.js`,
    `/${ASSETS}/qa-missing.css`,
    `/${ASSETS}/nope/qa.js`,
    `/${ASSETS}`,
    // The retired directory: browsers may hold HTML under these URLs.
    `/assets/${chunk}`,
    "/assets/qa-missing.css",
  ]) {
    const res = await head(path, ["x-content-type-options"]);
    assert.equal(res.status, 404, path);
    assert.equal(res.headers.get("cache-control"), "no-store", path);
    assert.doesNotMatch(res.headers.get("content-type") ?? "", /html/, path);
    const body = await (await fetch(origin + path)).text();
    assert.equal(body, "Not found", path);
  }
});

test("a page stays private and revalidated", async () => {
  const res = await head("/does-not-exist-page");
  assert.equal(res.headers.get("cache-control"), "private, no-cache");
});

test("the association documents keep their public lifetime", async () => {
  for (const path of [
    "/.well-known/apple-app-site-association",
    "/.well-known/assetlinks.json",
  ]) {
    const res = await head(path);
    assert.equal(res.status, 200, path);
    assert.equal(res.headers.get("cache-control"), "public, max-age=300");
    assert.match(res.headers.get("content-type"), /^application\/json/);
  }
});
