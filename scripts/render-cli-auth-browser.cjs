#!/usr/bin/env node
/*
 * Drive the browser half of the official Render CLI device flow. This helper
 * deliberately starts a real Chrome process; the shell verifier owns the CLI,
 * token assertions, and cleanup. `playwright-core` is supplied through
 * NODE_PATH by scripts/render-cli-auth-e2e.sh so the product does not gain a
 * runtime browser dependency.
 */
const { chromium } = require("playwright-core");

const [, , verificationURL] = process.argv;
if (!verificationURL) {
  console.error("usage: render-cli-auth-browser.cjs <verification-url>");
  process.exit(2);
}

async function credentialsFromStdin() {
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  const [email, password, end] = Buffer.concat(chunks).toString().split("\0");
  if (!email || !password || end !== "") {
    throw new Error(
      "expected NUL-delimited email and password on standard input",
    );
  }
  return { email, password };
}

const executablePath =
  process.env.CHROME_BIN ||
  (process.platform === "darwin"
    ? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    : undefined);

// Diagnostics never print page text, query strings, or field values — only
// the path and the shape of the form on it.
async function debugPage(page, path) {
  if (process.env.DEBUG_BROWSER !== "1") return;
  await page.waitForTimeout(1_000);
  const inputs = await page.locator("input").evaluateAll((nodes) =>
    nodes.map((node) => ({
      name: node.getAttribute("name"),
      type: node.getAttribute("type"),
    })),
  );
  console.error(`page ${path} inputs: ${JSON.stringify(inputs)}`);
}

// submitPasswordLogin fills and submits the password method. On a Hydra
// re-authentication (`?login_challenge=…`) the password field starts collapsed
// behind the method's own button and the identifier is present but hidden, so
// neither is assumed visible: the method button is clicked first when the field
// is not there, and the identifier is filled only when it is fillable. On a
// first login both are visible and the extra click never happens.
async function submitPasswordLogin(page, email, password) {
  const method = page.locator('button[name="method"][value="password"]');
  const passwordField = page.locator('input[name="password"]');
  await method.first().waitFor({ state: "attached", timeout: 30_000 });
  if (
    !(await passwordField
      .first()
      .isVisible()
      .catch(() => false))
  ) {
    await method.first().click();
    await passwordField.first().waitFor({ state: "visible", timeout: 30_000 });
  }
  const identifier = page.locator('input[name="identifier"]');
  if (
    await identifier
      .first()
      .isVisible()
      .catch(() => false)
  ) {
    await identifier.first().fill(email);
  }
  await passwordField.first().fill(password);
  // The password method button specifically — production's login page also
  // renders social-login submit buttons (Sign in with GitHub), so a bare
  // button[type="submit"] is ambiguous there.
  await method.first().click();
}

// authorizeDevice clicks "Authorize device" on the confirmation page. The
// selector is the form's action rather than its label so it survives both a
// locale change and the pre-hydration render (the form POSTs natively).
async function authorizeDevice(page) {
  const authorize = page.locator(
    'form[action="/auth/device"] button[type="submit"]',
  );
  await authorize.first().waitFor({ state: "visible", timeout: 30_000 });
  await authorize.first().click();
}

async function approveConsent(page) {
  const approve = page.locator('button[value="approve"]');
  await approve.first().waitFor({ state: "visible", timeout: 30_000 });
  await approve.first().click();
}

(async () => {
  const { email, password } = await credentialsFromStdin();
  const browser = await chromium.launch({
    executablePath,
    headless: process.env.HEADED !== "1",
  });
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    if (process.env.DEBUG_BROWSER === "1") {
      page.on("response", (response) => {
        const url = new URL(response.url());
        if (
          url.hostname === "localhost" &&
          (url.pathname.startsWith("/oauth2/") ||
            url.pathname.startsWith("/auth/") ||
            url.pathname.startsWith("/self-service/"))
        ) {
          console.error(`response: ${response.status()} ${url.pathname}`);
        }
      });
    }
    await page.goto(verificationURL, { waitUntil: "domcontentloaded" });

    // The flow's legs differ per environment and per session state, so drive
    // whichever page is on screen instead of a fixed sequence. Production
    // inserts an "Authorize this device" confirmation and then demands a
    // re-authentication before it skips consent; a local stack may show
    // neither. Each leg is idempotent, so an environment that omits one simply
    // never enters that branch (w9/062).
    for (let leg = 0; leg < 8; leg += 1) {
      await page.waitForURL(
        /\/auth\/(?:login|device|device\/success|consent)(?:\?|$)/,
        { timeout: 30_000 },
      );
      const before = page.url();
      const path = new URL(before).pathname;
      await debugPage(page, path);
      if (path === "/auth/device/success") break;
      if (path === "/auth/login") {
        await submitPasswordLogin(page, email, password);
      } else if (path === "/auth/device") {
        await authorizeDevice(page);
      } else {
        await approveConsent(page);
      }
      // A leg's submit navigates. Waiting for the URL to actually change keeps
      // the next iteration from re-reading the page it just left and acting on
      // a control that is already gone. A leg that legitimately stays put falls
      // through to the next iteration, which the loop bound covers.
      await page
        .waitForURL((url) => url.toString() !== before, { timeout: 30_000 })
        .catch(() => {});
    }

    await page.waitForURL(/\/auth\/device\/success(?:\?|$)/, {
      timeout: 30_000,
    });
    await page
      .getByText(/render cli|connected/i)
      .first()
      .waitFor({
        state: "visible",
        timeout: 10_000,
      });
    console.log("authorized browser session");
  } finally {
    await browser.close();
  }
})().catch((error) => {
  console.error(error instanceof Error ? error.stack : String(error));
  process.exit(1);
});
