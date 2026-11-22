import { afterEach, beforeAll, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import i18n, { ensureLanguage } from "@/i18n/init";
import { SUPPORTED_LANGUAGES } from "@/i18n/config";
import { ComputePlanList, type ComputePlanTier } from "../compute-plan-list";

// w6/m459: a Key Value plan card states its connection limit on a line of its
// own under the RAM, as Render's /new/redis cards do ("Free … 50 Connections");
// a tier without one (services, Postgres, an older catalog) shows no line.
const tiers: ComputePlanTier[] = [
  {
    id: "free",
    name: "Free",
    cpu: "0.1",
    memory: "128Mi",
    connectionLimit: 50,
  },
  {
    id: "standard",
    name: "Standard",
    cpu: "1",
    memory: "1Gi",
    connectionLimit: 1000,
  },
  {
    id: "pro_ultra",
    name: "Pro Ultra",
    cpu: "4",
    memory: "40Gi",
    connectionLimit: 40000,
    available: false,
    unavailableReason: "paused",
  },
  { id: "web", name: "Web", cpu: "0.5", memory: "512Mi" },
];

function renderGrid() {
  return render(
    <ComputePlanList
      instanceTypes={tiers}
      value="free"
      ariaLabel="Instance type"
      onChange={() => {}}
    />,
  );
}

const card = (name: RegExp) => screen.getByRole("radio", { name });

describe("ComputePlanList connection limit", () => {
  beforeAll(async () => {
    for (const lang of SUPPORTED_LANGUAGES) await ensureLanguage(lang);
  });
  afterEach(async () => {
    await act(() => i18n.changeLanguage("en"));
  });

  // A row is the radio, the name with its specs, and the price; the specs are
  // CPU, RAM and — for a tier that has one — the connection limit.
  const specs = (row: HTMLElement) => row.children[1].children[1];

  it("gives each plan's limit a spec of its own, a paused plan's too", () => {
    renderGrid();
    const free = card(/Free/);
    expect(free.children).toHaveLength(3);
    expect(specs(free).children).toHaveLength(3);
    expect(specs(free).children[2]).toHaveTextContent(/^50 connections$/);
    expect(card(/Standard/)).toHaveTextContent("1,000 connections");
    expect(card(/Pro Ultra/)).toHaveTextContent("40,000 connections");
  });

  it("shows no line for a tier without a limit", () => {
    renderGrid();
    const web = card(/Web/);
    expect(specs(web).children).toHaveLength(2);
    expect(web).not.toHaveTextContent(/connection/i);
  });

  it("formats the count in the dashboard's language", async () => {
    await act(() => i18n.changeLanguage("de"));
    renderGrid();
    expect(card(/Standard/)).toHaveTextContent("1.000 Verbindungen");
    expect(card(/Free/)).toHaveTextContent("50 Verbindungen");
  });

  // Every Render Key Value limit (compute-plans, read 2026-11-17) reads in
  // every language: its own words around the locale's number, never the key.
  it.each(SUPPORTED_LANGUAGES)("reads every Render limit in %s", (lang) => {
    for (const n of [50, 250, 1000, 3000, 5000, 10000, 20000, 40000, 50000]) {
      const formatted = new Intl.NumberFormat(lang).format(n);
      const text = i18n.t("common.planConnections", {
        count: n,
        formatted,
        lng: lang,
      });
      expect(text).toContain(formatted);
      expect(text).not.toContain("common.planConnections");
      if (lang !== "en") expect(text).not.toMatch(/connections/);
    }
  });
});
