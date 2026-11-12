import { Suspense } from "react";
import { describe, expect, it, vi } from "vitest";
import i18n from "@/i18n/init";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import {
  BROWSER_ENVIRONMENT,
  CHINESE_BROWSER_ENVIRONMENT,
  SERVER_ENVIRONMENT,
  hydrateAcrossEnvironments,
} from "@/test/hydration";

// The probe renders on a "server" (UTC, English, 23:59:30 on 31 October) and
// hydrates in a "browser" (Shanghai, English, a minute later). These tests pin
// that it reports the mismatches that reached production (w6/049, w6/m107,
// w21/013) and stays quiet for the patterns that fixed them.

const INSTANT = Date.UTC(2026, 9, 15, 12, 0, 0);

// w6/m107: a formatter that reads the runtime's zone for a fixed instant.
function LocalTime() {
  return <p>{new Date(INSTANT).toLocaleString("en-US")}</p>;
}

function UtcTime() {
  return (
    <p>{new Date(INSTANT).toLocaleString("en-US", { timeZone: "UTC" })}</p>
  );
}

// w6/049: ChargesCard's `now = new Date()` default parameter, read once on the
// server and again on the client, fed a month label and an elapsed-time ratio.
function ClockCard() {
  const now = new Date();
  const month = now.toLocaleString("en-US", { month: "long", timeZone: "UTC" });
  const ratio = (now.getTime() % 1_000_000) / 1_000_000;
  return (
    <p>
      Projected total for {month}: {ratio.toFixed(6)}
    </p>
  );
}

// The fix pattern the repo uses: clock-dependent text appears after hydration.
function ClockCardAfterMount() {
  const hydrated = useIsHydrated();
  return <p>{hydrated ? new Date().toISOString() : "…"}</p>;
}

// w21/013: a form's default time zone read from the runtime.
function ZoneDefault() {
  return <p>{Intl.DateTimeFormat().resolvedOptions().timeZone}</p>;
}

function ZoneDefaultAfterMount() {
  const hydrated = useIsHydrated();
  return (
    <p>{hydrated ? Intl.DateTimeFormat().resolvedOptions().timeZone : "UTC"}</p>
  );
}

function StoredLanguage() {
  const stored = localStorage.getItem("i18nextLng") ?? "none";
  return <p>{`${stored} / ${document.cookie || "no cookie"}`}</p>;
}

describe("hydrateAcrossEnvironments", () => {
  it("runs every test in UTC unless a probe says otherwise", () => {
    expect(Intl.DateTimeFormat().resolvedOptions().timeZone).toBe("UTC");
    expect(new Date(INSTANT).getHours()).toBe(12);
  });

  it("reports a time-zone-dependent render, and not a zone-explicit one", async () => {
    const zoned = await hydrateAcrossEnvironments(() => <LocalTime />);
    expect(zoned.recovered).not.toHaveLength(0);
    expect(zoned.html).toContain("12:00:00 PM");

    const pinned = await hydrateAcrossEnvironments(() => <UtcTime />);
    expect(pinned.recovered).toHaveLength(0);
  });

  it("reports a clock read across the boundary, and not one after hydration (w6/049)", async () => {
    const live = await hydrateAcrossEnvironments(() => <ClockCard />);
    expect(live.recovered).not.toHaveLength(0);
    expect(live.html).toContain("October");

    const afterMount = await hydrateAcrossEnvironments(() => (
      <ClockCardAfterMount />
    ));
    expect(afterMount.recovered).toHaveLength(0);
  });

  it("reports a default time zone read from the runtime, and not one set after hydration (w21/013)", async () => {
    const live = await hydrateAcrossEnvironments(() => <ZoneDefault />);
    expect(live.recovered).not.toHaveLength(0);

    const afterMount = await hydrateAcrossEnvironments(() => (
      <ZoneDefaultAfterMount />
    ));
    expect(afterMount.recovered).toHaveLength(0);
  });

  it("expresses the missing-cookie, stored-language case (w21/054)", async () => {
    const probe = await hydrateAcrossEnvironments(() => <StoredLanguage />, {
      server: { ...SERVER_ENVIRONMENT, storage: { i18nextLng: "zh" } },
      client: { ...BROWSER_ENVIRONMENT, cookies: { i18nextLng: "zh" } },
    });
    expect(probe.html).toContain("zh / no cookie");
    expect(probe.recovered).not.toHaveLength(0);
  });

  it("hydrates a translated tree the same on both sides in the Chinese environment", async () => {
    const probe = await hydrateAcrossEnvironments(
      () => <p>{i18n.t("common.navSettings")}</p>,
      {
        server: { ...SERVER_ENVIRONMENT, language: "zh" },
        client: CHINESE_BROWSER_ENVIRONMENT,
      },
    );
    expect(probe.html).toContain(i18n.t("common.navSettings", { lng: "zh" }));
    expect(probe.recovered).toHaveLength(0);
  });

  it("reports a server pass that errored inside a boundary and hydrates nothing", async () => {
    const quiet = vi.spyOn(console, "error").mockImplementation(() => {});
    function Boom(): never {
      throw new Error("no data on the server");
    }
    const probe = await hydrateAcrossEnvironments(() => (
      <Suspense fallback={<p>waiting</p>}>
        <Boom />
      </Suspense>
    ));
    quiet.mockRestore();
    expect(probe.serverErrored).toBe(true);
    expect(probe.recovered).toHaveLength(0);
    expect(probe.failed).toHaveLength(0);
  });

  it("puts the time zone, clock, language, cookies and storage back", async () => {
    const zone = process.env.TZ;
    const language = i18n.language;
    const storage = globalThis.localStorage;
    const before = Date.now();

    await hydrateAcrossEnvironments(() => <StoredLanguage />, {
      server: { ...SERVER_ENVIRONMENT, storage: { i18nextLng: "zh" } },
      client: CHINESE_BROWSER_ENVIRONMENT,
    });

    expect(process.env.TZ).toBe(zone);
    expect(i18n.language).toBe(language);
    expect(globalThis.localStorage).toBe(storage);
    expect(document.cookie).not.toContain("i18nextLng");
    expect(Math.abs(Date.now() - before)).toBeLessThan(60_000);
    expect(navigator.language).not.toBe("zh");
  });
});
