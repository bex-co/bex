import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { TranslationEntry } from "@/i18n";
import { WORKSPACE_PLAN_CATALOG } from "@/features/workspaces/types";
import en from "@/features/workspaces/locales/en";
import zh from "@/features/workspaces/locales/zh";
import ko from "@/features/workspaces/locales/ko";
import ja from "@/features/workspaces/locales/ja";
import es from "@/features/workspaces/locales/es";
import pt from "@/features/workspaces/locales/pt";
import ru from "@/features/workspaces/locales/ru";
import de from "@/features/workspaces/locales/de";

// The plan cards' billing labels quote the workspace fees bex-api lists from
// lego/backend/internal/pricing/pricing.yaml (ADR030), and say what one fee
// covers: Scale's is `per: organization` (w7/m464, ADR115 D2), so its label
// says so in every language, as Enterprise's does. A fee or `per` change that
// forgets the copy fails here.
const PRICING = readFileSync(
  `${process.cwd()}/../lego/backend/internal/pricing/pricing.yaml`,
  "utf8",
);
const start = PRICING.indexOf("\nworkspace:\n");
const WORKSPACE_FEES = PRICING.slice(start, PRICING.indexOf("\n\n", start));
const FEES = [
  ...WORKSPACE_FEES.matchAll(
    /- plan: (\w+)\n\s+usdPerMonth: ([\d.]+)[^\n]*\n\s+per: (\w+)/g,
  ),
].map(([, plan, usd, per]) => ({ plan: plan!, usd: Number(usd), per: per! }));

/** Each language's word for the organization one fee covers. */
const LOCALES: Array<[string, Record<string, TranslationEntry>, string]> = [
  ["en", en, "per organization"],
  ["zh", zh, "组织"],
  ["ko", ko, "조직"],
  ["ja", ja, "組織"],
  ["es", es, "organización"],
  ["pt", pt, "organização"],
  ["ru", ru, "организаци"],
  ["de", de, "Organisation"],
];

function billingLabel(
  catalog: Record<string, TranslationEntry>,
  plan: string,
): string {
  const key = WORKSPACE_PLAN_CATALOG.find((p) => p.id === plan)!.billingKey;
  return catalog[key]!.message;
}

describe("workspace plan billing labels match pricing.yaml", () => {
  it("reads every listed workspace fee", () => {
    expect(FEES.map((fee) => fee.plan)).toEqual(["hobby", "pro", "scale"]);
  });

  it.each(LOCALES)("%s quotes each fee", (_, catalog) => {
    for (const { plan, usd } of FEES) {
      const amount = Number.isInteger(usd) ? `${usd}` : usd.toFixed(2);
      // pt writes the decimal comma ("$349,30").
      const figures = [amount, amount.replace(".", ",")];
      const label = billingLabel(catalog, plan);
      expect(figures.some((figure) => label.includes(`$${figure}`))).toBe(true);
    }
  });

  it.each(LOCALES)(
    "%s says per organization exactly where one fee covers the organization",
    (_, catalog, organization) => {
      for (const { plan, per } of FEES) {
        expect(billingLabel(catalog, plan).includes(organization)).toBe(
          per === "organization",
        );
      }
      // Enterprise lists no fee (custom terms), but it holds an organization
      // as Scale does (core.OrganizationPlan).
      expect(billingLabel(catalog, "enterprise")).toContain(organization);
    },
  );
});
