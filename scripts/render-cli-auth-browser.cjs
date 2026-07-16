#!/usr/bin/env node
/*
 * Drive the browser half of the official Render CLI device flow. This helper
 * deliberately starts a real Chrome process; the shell verifier owns the CLI,
 * token assertions, and cleanup. `playwright-core` is supplied through
 * NODE_PATH by scripts/render-cli-auth-e2e.sh so the product does not gain a
 * runtime browser dependency.
 */
const { chromium } = require("playwright-core");

const [, , verificationURL, email, password] = process.argv;
if (!verificationURL || !email || !password) {
  console.error(
    "usage: render-cli-auth-browser.cjs <verification-url> <email> <password>",
  );
  process.exit(2);
}

const executablePath =
  process.env.CHROME_BIN ||
  (process.platform === "darwin"
    ? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    : undefined);

(async () => {
  const browser = await chromium.launch({
    executablePath,
    headless: process.env.HEADED !== "1",
  });
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    await page.goto(verificationURL, { waitUntil: "domcontentloaded" });

    await page.waitForURL(/\/auth\/login(?:\?|$)/, { timeout: 30_000 });
    if (process.env.DEBUG_BROWSER === "1") {
      await page.waitForTimeout(2_000);
      console.error(`browser url: ${page.url()}`);
      console.error(
        `inputs: ${JSON.stringify(await page.locator("input").evaluateAll((nodes) => nodes.map((node) => ({ name: node.getAttribute("name"), type: node.getAttribute("type") }))))}`,
      );
      console.error(`body: ${(await page.locator("body").innerText()).slice(0, 1_000)}`);
    }
    await page.locator('input[name="identifier"]').fill(email);
    await page.locator('input[name="password"]').fill(password);
    // The password method button specifically — production's login page also
    // renders social-login submit buttons (Sign in with GitHub), so a bare
    // button[type="submit"] is ambiguous there.
    await page.locator('button[name="method"][value="password"]').click();

    await page.waitForURL(/\/auth\/(?:consent|device\/success)(?:\?|$)/, {
      timeout: 30_000,
    });
    if (new URL(page.url()).pathname === "/auth/consent") {
      const approve = page.locator('button[value="approve"]');
      await approve.waitFor({ state: "visible" });
      await approve.click();
    }

    await page.waitForURL(/\/auth\/device\/success(?:\?|$)/, {
      timeout: 30_000,
    });
    await page.getByText(/render cli|connected/i).first().waitFor({
      state: "visible",
      timeout: 10_000,
    });
    console.log(`authorized ${email}`);
  } finally {
    await browser.close();
  }
})().catch((error) => {
  console.error(error instanceof Error ? error.stack : String(error));
  process.exit(1);
});
