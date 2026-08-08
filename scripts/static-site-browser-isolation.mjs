#!/usr/bin/env node
// Real-browser Public Suffix and origin-isolation probe (w7/m54).
//
// No live tenant site is modified. A loopback HTTPS server is reached through
// Chrome host mappings for two sibling hosts. The browser, not a PSL library,
// decides whether Domain=<suffix> is legal. onrender.com is the reference; the
// candidate hosting suffix defaults to onbex.co and must reject parent cookies.
//
// Usage:
//   node scripts/static-site-browser-isolation.mjs
//   BEX_HOSTING_SUFFIX=hosting.example node scripts/static-site-browser-isolation.mjs
//   PSL_EXPECTED=absent node scripts/static-site-browser-isolation.mjs # diagnostic only
//   CHROME_BIN=/path/to/chrome node scripts/static-site-browser-isolation.mjs
import { execFileSync, spawn } from "node:child_process";
import fs from "node:fs";
import https from "node:https";
import os from "node:os";
import path from "node:path";

const expected = process.env.PSL_EXPECTED ?? "present";
if (!new Set(["report", "present", "absent"]).has(expected)) {
  throw new Error("PSL_EXPECTED must be report, present, or absent");
}
const hostingSuffix = process.env.BEX_HOSTING_SUFFIX ?? "onbex.co";
if (
  !/^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])$/.test(hostingSuffix) ||
  !hostingSuffix.includes(".")
) {
  throw new Error("BEX_HOSTING_SUFFIX must be a canonical DNS suffix");
}

const chromeCandidates = [
  process.env.CHROME_BIN,
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  "/Applications/Chromium.app/Contents/MacOS/Chromium",
  "/usr/bin/google-chrome",
  "/usr/bin/chromium",
  "/usr/bin/chromium-browser",
].filter(Boolean);
const chrome = chromeCandidates.find((candidate) => fs.existsSync(candidate));
if (!chrome) throw new Error("Chrome/Chromium not found; set CHROME_BIN");

const scratch = fs.mkdtempSync(
  path.join(os.tmpdir(), "bex-browser-isolation-"),
);
const cert = path.join(scratch, "cert.pem");
const key = path.join(scratch, "key.pem");
const profile = path.join(scratch, "chrome-profile");
fs.mkdirSync(profile);
execFileSync(
  "openssl",
  [
    "req",
    "-x509",
    "-newkey",
    "rsa:2048",
    "-nodes",
    "-days",
    "1",
    "-subj",
    "/CN=static-site-security.invalid",
    "-keyout",
    key,
    "-out",
    cert,
  ],
  { stdio: "ignore" },
);

const server = https.createServer(
  { cert: fs.readFileSync(cert), key: fs.readFileSync(key) },
  (request, response) => {
    if (request.url === "/sw.js") {
      response.setHeader("content-type", "text/javascript; charset=utf-8");
      response.setHeader("cache-control", "no-store");
      response.end("self.addEventListener('fetch', () => {});");
      return;
    }
    response.setHeader("content-type", "text/html; charset=utf-8");
    response.setHeader("cache-control", "no-store");
    response.end(
      "<!doctype html><meta charset=utf-8><title>static-site isolation probe</title>",
    );
  },
);
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const { port } = server.address();

const chromeProcess = spawn(
  chrome,
  [
    "--headless=new",
    "--disable-background-networking",
    "--disable-component-update",
    "--disable-default-apps",
    "--disable-extensions",
    "--disable-gpu",
    "--disable-sync",
    "--ignore-certificate-errors",
    "--no-first-run",
    "--no-proxy-server",
    "--no-sandbox",
    "--remote-debugging-port=0",
    `--user-data-dir=${profile}`,
    `--host-resolver-rules=MAP tenant-a.${hostingSuffix} 127.0.0.1, MAP tenant-b.${hostingSuffix} 127.0.0.1, MAP tenant-a.onrender.com 127.0.0.1, MAP tenant-b.onrender.com 127.0.0.1`,
    "about:blank",
  ],
  { stdio: "ignore" },
);

function delay(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

async function waitForDevTools() {
  const activePort = path.join(profile, "DevToolsActivePort");
  for (let attempt = 0; attempt < 200; attempt += 1) {
    if (fs.existsSync(activePort)) {
      const [debugPort, browserPath] = fs
        .readFileSync(activePort, "utf8")
        .trim()
        .split("\n");
      return `ws://127.0.0.1:${debugPort}${browserPath}`;
    }
    if (chromeProcess.exitCode !== null)
      throw new Error("Chrome exited before DevTools was ready");
    await delay(50);
  }
  throw new Error("Chrome DevTools did not become ready within 10 seconds");
}

class CDP {
  constructor(webSocket) {
    this.webSocket = webSocket;
    this.nextID = 1;
    this.pending = new Map();
    this.waiters = [];
    webSocket.addEventListener("message", ({ data }) =>
      this.onMessage(JSON.parse(data)),
    );
  }

  onMessage(message) {
    if (message.id) {
      const pending = this.pending.get(message.id);
      if (!pending) return;
      this.pending.delete(message.id);
      if (message.error) pending.reject(new Error(message.error.message));
      else pending.resolve(message.result);
      return;
    }
    const index = this.waiters.findIndex(
      (waiter) =>
        waiter.method === message.method &&
        waiter.sessionID === message.sessionId,
    );
    if (index >= 0) this.waiters.splice(index, 1)[0].resolve(message.params);
  }

  send(method, params = {}, sessionID) {
    const id = this.nextID++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.webSocket.send(
        JSON.stringify({ id, method, params, sessionId: sessionID }),
      );
    });
  }

  event(method, sessionID) {
    return new Promise((resolve) =>
      this.waiters.push({ method, sessionID, resolve }),
    );
  }
}

async function connect(url) {
  const webSocket = new WebSocket(url);
  await new Promise((resolve, reject) => {
    webSocket.addEventListener("open", resolve, { once: true });
    webSocket.addEventListener("error", reject, { once: true });
  });
  return new CDP(webSocket);
}

async function navigate(cdp, sessionID, url) {
  const loaded = cdp.event("Page.loadEventFired", sessionID);
  const result = await cdp.send("Page.navigate", { url }, sessionID);
  if (result.errorText)
    throw new Error(`navigation failed: ${result.errorText}`);
  await loaded;
}

async function evaluate(cdp, sessionID, expression) {
  const result = await cdp.send(
    "Runtime.evaluate",
    { expression, awaitPromise: true, returnByValue: true },
    sessionID,
  );
  if (result.exceptionDetails) {
    throw new Error(
      result.exceptionDetails.exception?.description ??
        "browser evaluation failed",
    );
  }
  return result.result.value;
}

async function checkSuffix(cdp, suffix, wantPSL) {
  const { browserContextId } = await cdp.send("Target.createBrowserContext");
  const { targetId } = await cdp.send("Target.createTarget", {
    url: "about:blank",
    browserContextId,
  });
  const { sessionId } = await cdp.send("Target.attachToTarget", {
    targetId,
    flatten: true,
  });
  await cdp.send("Page.enable", {}, sessionId);
  await cdp.send("Runtime.enable", {}, sessionId);

  await navigate(cdp, sessionId, `https://tenant-a.${suffix}:${port}/`);
  const tenantA = await evaluate(
    cdp,
    sessionId,
    `(async () => {
      document.cookie = ${JSON.stringify(`bex_parent_probe=tenant-a; Domain=${suffix}; Path=/; Secure; SameSite=Lax`)};
      localStorage.setItem("bex_origin_probe", "tenant-a");
      const registration = await navigator.serviceWorker.register("/sw.js").catch(() => null);
      return {
        cookie: document.cookie,
        storage: localStorage.getItem("bex_origin_probe"),
        workers: registration ? 1 : 0,
        origin: location.origin,
      };
    })()`,
  );

  await navigate(cdp, sessionId, `https://tenant-b.${suffix}:${port}/`);
  const tenantB = await evaluate(
    cdp,
    sessionId,
    `(async () => ({
      cookie: document.cookie,
      storage: localStorage.getItem("bex_origin_probe"),
      workers: (await navigator.serviceWorker.getRegistrations()).length,
      origin: location.origin,
    }))()`,
  );

  const parentCrossed = tenantB.cookie.includes("bex_parent_probe=tenant-a");
  const failures = [];
  if (wantPSL !== null && parentCrossed === wantPSL) {
    failures.push(
      wantPSL
        ? "browser accepted a cookie scoped to the public suffix"
        : "browser rejected the expected non-PSL cookie behavior",
    );
  }
  if (tenantB.storage !== null)
    failures.push("localStorage crossed sibling origins");
  if (tenantB.workers !== 0)
    failures.push("Service Worker registration crossed sibling origins");
  if (tenantA.workers !== 1)
    failures.push("control Service Worker did not register on tenant A");
  if (tenantA.origin === tenantB.origin)
    failures.push("sibling hosts collapsed to one origin");

  await cdp.send("Target.closeTarget", { targetId });
  await cdp.send("Target.disposeBrowserContext", { browserContextId });

  if (failures.length) {
    console.error(`FAIL  ${suffix}: ${failures.join("; ")}`);
    return false;
  }
  const cookieResult = parentCrossed ? "crossed to sibling" : "rejected";
  const expectation = wantPSL === null ? "reported, not gated" : "expected";
  console.log(
    `PASS  ${suffix}: parent cookie ${cookieResult} (${expectation}); storage and Service Worker stayed origin-local`,
  );
  return true;
}

let cdp;
try {
  cdp = await connect(await waitForDevTools());
  const renderOK = await checkSuffix(cdp, "onrender.com", true);
  const bexExpectation = expected === "report" ? null : expected === "present";
  const bexOK = await checkSuffix(cdp, hostingSuffix, bexExpectation);
  if (!renderOK || !bexOK) process.exitCode = 1;
} finally {
  if (cdp) await cdp.send("Browser.close").catch(() => {});
  for (
    let attempt = 0;
    attempt < 50 && chromeProcess.exitCode === null;
    attempt += 1
  )
    await delay(20);
  if (chromeProcess.exitCode === null) chromeProcess.kill("SIGKILL");
  server.closeAllConnections();
  await new Promise((resolve) => server.close(resolve));
  fs.rmSync(scratch, { recursive: true, force: true });
}
