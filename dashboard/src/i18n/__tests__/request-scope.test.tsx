import { describe, it, expect, afterEach } from "vitest";
import { render, within } from "@testing-library/react";
import { I18nextProvider, useTranslation } from "react-i18next";
import i18n, {
  createRequestI18nInstance,
  ensureLanguageOn,
  switchLanguage,
} from "@/i18n/init";
import {
  runWithRequestI18n,
  getActiveI18nOnServer,
} from "@/i18n/request-scope/server";
import {
  i18nBootstrapScript,
  resolveShellLanguage,
} from "@/i18n/client-bootstrap";
import zhResources from "@/i18n/resources-zh";

// A known key that exists in both catalogs, so a render's language is
// unambiguous from its text. `test/setup.ts` preloads the zh bundle onto the
// shared singleton, but these per-request instances load it themselves.
const KEY = "common.notFoundTitle";
const EN = "Page not found";
const ZH = "页面未找到";

function Probe() {
  const { t } = useTranslation();
  return <span data-testid="t">{t(KEY)}</span>;
}

const tick = () => new Promise<void>((r) => setTimeout(r, 0));

describe("w6/m103 Bug B — SSR per-request i18n isolation", () => {
  it("two overlapping requests keep their own language — no shared-singleton clobber", async () => {
    const reqZh = createRequestI18nInstance();
    const reqEn = createRequestI18nInstance();

    // Interleave setup the way concurrent SSR requests do: the zh request
    // switches to zh, then an en request runs its own changeLanguage "in
    // between". With the pre-fix shared singleton this second call moved the
    // language out from under the zh render; per-request instances don't.
    await ensureLanguageOn(reqZh, "zh");
    await reqZh.changeLanguage("zh");
    await reqEn.changeLanguage("en");

    expect(reqZh.language).toBe("zh");
    expect(reqEn.language).toBe("en");

    const zhTree = render(
      <I18nextProvider i18n={reqZh}>
        <Probe />
      </I18nextProvider>,
    );
    expect(within(zhTree.container).getByTestId("t").textContent).toBe(ZH);

    const enTree = render(
      <I18nextProvider i18n={reqEn}>
        <Probe />
      </I18nextProvider>,
    );
    expect(within(enTree.container).getByTestId("t").textContent).toBe(EN);

    // The shared client singleton is untouched by either request instance.
    expect(i18n).not.toBe(reqZh);
    expect(i18n).not.toBe(reqEn);
  });

  it("getActiveI18nOnServer returns the request-scoped instance, singleton outside a scope", () => {
    const req = createRequestI18nInstance();
    expect(runWithRequestI18n(req, () => getActiveI18nOnServer())).toBe(req);
    // Outside any request scope it falls back to the shared singleton.
    expect(getActiveI18nOnServer()).toBe(i18n);
  });

  it("three concurrent request scopes do not leak across await points (AsyncLocalStorage)", async () => {
    // Three overlapping requests, default language mixed in among non-default
    // ones — the generalization past the two-language minimal repro (t006).
    const wanted = ["zh", "en", "zh"];
    const instances = await Promise.all(
      wanted.map(async (lng) => {
        const inst = createRequestI18nInstance();
        await ensureLanguageOn(inst, lng);
        await inst.changeLanguage(lng);
        return inst;
      }),
    );

    // Every scope yields to the event loop before reading the active instance,
    // so a leak would surface as a wrong language for at least one.
    const langs = await Promise.all(
      instances.map((inst, i) =>
        runWithRequestI18n(inst, async () => {
          await tick();
          return getActiveI18nOnServer().language === wanted[i];
        }),
      ),
    );
    expect(langs).toEqual([true, true, true]);
  });

  it("ensureLanguageOn is idempotent and no-ops for the default language", async () => {
    const inst = createRequestI18nInstance();
    await ensureLanguageOn(inst, "zh");
    const bundle = inst.getResourceBundle("zh", "translation");
    // A second call short-circuits on hasResourceBundle — same object, not re-added.
    await ensureLanguageOn(inst, "zh");
    expect(inst.getResourceBundle("zh", "translation")).toBe(bundle);
    // The default language is already in the entry bundle — always a no-op.
    await ensureLanguageOn(inst, "en");
    expect(inst.hasResourceBundle("en", "translation")).toBe(true);
  });
});

describe("w6/m103 — switchLanguage (the one client switch path)", () => {
  // Restore setup.ts's baseline (zh preloaded, language en) after mutating it.
  afterEach(async () => {
    i18n.addResourceBundle("zh", "translation", zhResources, true, true);
    await i18n.changeLanguage("en");
  });

  it("registers the lazy catalog before changing language (Bug A contract)", async () => {
    // Strip the setup.ts preload so the switch must honor the load-first
    // contract — the exact condition Bug A regressed under.
    i18n.removeResourceBundle("zh", "translation");
    await i18n.changeLanguage("en");
    expect(i18n.hasResourceBundle("zh", "translation")).toBe(false);

    await switchLanguage("zh");

    expect(i18n.hasResourceBundle("zh", "translation")).toBe(true);
    expect(i18n.language).toBe("zh");
  });
});

describe("w6/m103 — client hydration bootstrap", () => {
  it("emits no bootstrap for the default language", () => {
    expect(i18nBootstrapScript("en")).toBeNull();
  });

  // Regression: a navigation into a `pendingMs: 0` detail route republishes
  // the root match with its pre-`beforeLoad` context (no `language`) while the
  // session fetch is in flight; the shell used to pass that `undefined`
  // straight into `getResourceBundle` (TypeError in i18next's getResource).
  it("resolveShellLanguage falls back to the active language when root context has none", async () => {
    expect(resolveShellLanguage(undefined)).toBe("en");
    expect(() =>
      i18nBootstrapScript(resolveShellLanguage(undefined)),
    ).not.toThrow();

    // A zh session mid-navigation keeps rendering zh (stable <html lang> and
    // a still-present bootstrap script), not a flash of the default.
    await switchLanguage("zh");
    try {
      expect(resolveShellLanguage(undefined)).toBe("zh");
      expect(i18nBootstrapScript(resolveShellLanguage(undefined))).toContain(
        '"lng":"zh"',
      );
    } finally {
      await i18n.changeLanguage("en");
    }

    // With context present it is authoritative, not the active instance.
    expect(resolveShellLanguage("zh")).toBe("zh");
  });

  it("stamps the active catalog onto the global and stays script-safe", () => {
    // A value that would break out of an inline <script> if left unescaped.
    i18n.addResourceBundle(
      "zh",
      "translation",
      { "test.xss": "</script><script>alert(1)</script>" },
      true,
      true,
    );
    try {
      const script = i18nBootstrapScript("zh");
      expect(script).toContain("globalThis.__BEX_I18N__=");
      expect(script).toContain('"lng":"zh"');
      // No raw "<" survives — every one is escaped to \u003c, so the payload
      // can never terminate the surrounding <script>.
      expect(script).not.toContain("<");
      expect(script).toContain("\\u003c/script>");
      // The escaped payload is still valid JSON (the < escapes parse back
      // to "<") producing the real catalog.
      const value = JSON.parse(
        script!.replace("globalThis.__BEX_I18N__=", ""),
      ) as { lng: string; catalog: Record<string, string> };
      expect(value.lng).toBe("zh");
      expect(value.catalog[KEY]).toBe(ZH);
      expect(value.catalog["test.xss"]).toBe(
        "</script><script>alert(1)</script>",
      );
    } finally {
      i18n.removeResourceBundle("zh", "translation");
      i18n.addResourceBundle("zh", "translation", zhResources, true, true);
    }
  });
});
