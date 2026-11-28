import { describe, expect, it } from "vitest";
import {
  cardMissing,
  downgradeConfirmationMatches,
  downgradeConfirmationPhrase,
  isDowngrade,
  planFromSearch,
  planSearchValue,
  readNext,
  updatePlanReturnPath,
  updatePlanSearch,
  upgradePlanTarget,
  validateUpdatePlanSearch,
} from "@/features/workspaces/lib/update-plan";
import { planComparisonRows } from "@/features/workspaces/lib/plan-comparison";

// Render's plan pages' URL grammar (w3/m708), read from its dashboard bundle's
// billing-confirm-plan-utils (docs/render-artifacts/workspace-plan-change.md).
describe("Render's ?plan= aliases", () => {
  it.each([
    ["hobby", "hobby"],
    ["team", "pro"],
    ["pro", "pro"],
    ["professional", "pro"],
    ["org", "scale"],
    ["scale", "scale"],
    ["organization", "scale"],
  ])("reads %s as %s", (value, plan) => {
    expect(planFromSearch(value)).toBe(plan);
  });

  it("has no alias for Enterprise, Render's Get in touch, or anything else", () => {
    for (const value of ["enterprise", "Pro", "", undefined, 3, ["pro"]]) {
      expect(planFromSearch(value)).toBeUndefined();
    }
  });

  it("writes Render's own spelling", () => {
    expect(planSearchValue("hobby")).toBe("hobby");
    expect(planSearchValue("pro")).toBe("professional");
    expect(planSearchValue("scale")).toBe("organization");
  });

  it("keeps a usable plan and next, and drops the rest", () => {
    expect(
      validateUpdatePlanSearch({ plan: "team", next: "/webhooks?x=1" }),
    ).toEqual({ plan: "professional", next: "/webhooks?x=1" });
    // Each key is there even when rejected: the router merges this over the
    // raw search, so an omitted key would reach the pages raw.
    for (const next of [7, "//evil.example", "https://evil.example/"]) {
      expect(validateUpdatePlanSearch({ plan: "gold", next })).toStrictEqual({
        plan: undefined,
        next: undefined,
      });
    }
  });
});

describe("next", () => {
  it("is kept only as a path on this dashboard", () => {
    expect(readNext("/")).toBe("/");
    expect(readNext("/workspace/settings#inbound-ip-rules")).toBe(
      "/workspace/settings#inbound-ip-rules",
    );
    for (const outside of [
      "//evil.example",
      "/\\evil.example",
      "https://evil.example/",
      "javascript:alert(1)",
      "relative/path",
      "",
      undefined,
    ]) {
      expect(readNext(outside)).toBeUndefined();
    }
  });

  it("names only what a link has", () => {
    expect(updatePlanSearch()).toEqual({});
    expect(updatePlanSearch("pro")).toEqual({ plan: "professional" });
    expect(updatePlanSearch("scale", "/webhooks")).toEqual({
      plan: "organization",
      next: "/webhooks",
    });
    expect(updatePlanSearch(undefined, "//evil.example")).toEqual({});
  });

  it("follows an upgrade link back to the page it was clicked on", () => {
    window.history.replaceState(null, "", "/project/prj-1?tab=x");
    expect(upgradePlanTarget("pro")).toEqual({
      to: "/billing/update-plan",
      search: { plan: "professional", next: "/project/prj-1" },
    });
  });
});

describe("where the plan pages return", () => {
  it("prefers next, with the page's own hash, as Render's does", () => {
    expect(updatePlanReturnPath("/webhooks", undefined)).toBe("/webhooks");
    expect(updatePlanReturnPath("/webhooks", undefined, "top")).toBe(
      "/webhooks#top",
    );
  });

  it("falls back to what the link handed over, then home", () => {
    expect(updatePlanReturnPath(undefined, "/billing")).toBe("/billing");
    expect(updatePlanReturnPath(undefined, "//evil.example")).toBe("/");
    expect(updatePlanReturnPath(undefined, undefined)).toBe("/");
  });
});

describe("downgrades and their typed phrase", () => {
  it("orders the plans Hobby, Pro, Scale, Enterprise", () => {
    expect(isDowngrade("pro", "hobby")).toBe(true);
    expect(isDowngrade("enterprise", "scale")).toBe(true);
    expect(isDowngrade("hobby", "pro")).toBe(false);
    expect(isDowngrade("pro", "pro")).toBe(false);
  });

  it("is Render's sudo phrase, the name trimmed and its spaces collapsed", () => {
    expect(downgradeConfirmationPhrase("  Tian   Pan's\tWorkspace ")).toBe(
      "sudo downgrade workspace Tian Pan's Workspace",
    );
  });

  it("matches what was typed under the same whitespace rule, and nothing else", () => {
    const phrase = downgradeConfirmationPhrase("acme");
    expect(downgradeConfirmationMatches(phrase, phrase)).toBe(true);
    expect(
      downgradeConfirmationMatches(
        "  sudo  downgrade workspace\tacme ",
        phrase,
      ),
    ).toBe(true);
    expect(
      downgradeConfirmationMatches("sudo downgrade workspace Acme", phrase),
    ).toBe(false);
    expect(
      downgradeConfirmationMatches("sudo downgrade workspace", phrase),
    ).toBe(false);
  });
});

describe("the card a paid plan needs", () => {
  const required = { paymentMethodRequired: true, paymentMethodReady: false };
  it("is missing for a paid plan while the platform requires one and none is on file", () => {
    expect(cardMissing("pro", required)).toBe(true);
    expect(cardMissing("scale", required)).toBe(true);
  });

  it("is never needed for Hobby, with a card on file, without the gate, or while unknown", () => {
    expect(cardMissing("hobby", required)).toBe(false);
    expect(
      cardMissing("pro", {
        paymentMethodRequired: true,
        paymentMethodReady: true,
      }),
    ).toBe(false);
    expect(
      cardMissing("pro", {
        paymentMethodRequired: false,
        paymentMethodReady: false,
      }),
    ).toBe(false);
    expect(cardMissing("pro", null)).toBe(false);
  });
});

describe("the confirmation's plan comparison", () => {
  const ids = (current: string, next: "hobby" | "pro" | "scale") =>
    planComparisonRows(current, next).map((row) => row.id);

  it("sets every limit Pro to Hobby shrinks side by side, old against new", () => {
    const rows = planComparisonRows("pro", "hobby");
    expect(rows.map((row) => row.id)).toEqual([
      "justForMe",
      "includedDomains",
      "twoEnvironments",
      "retentionDays",
      "requestLogs",
      "keptBuilds",
      "auditLogs",
    ]);
    expect(rows.find((row) => row.id === "retentionDays")).toEqual({
      id: "retentionDays",
      current: {
        key: "workspaces.planHighlightRetention",
        values: { days: 14 },
      },
      next: { key: "workspaces.planHighlightRetention", values: { days: 7 } },
    });
    expect(rows.find((row) => row.id === "auditLogs")?.next.missing).toBe(true);
    expect(
      rows.find((row) => row.id === "includedDomains")?.next.values,
    ).toEqual({ included: 2, price: "$0.175" });
  });

  it("names only what differs between Scale and Pro", () => {
    expect(ids("scale", "pro")).toEqual([
      "includedDomains",
      "retentionDays",
      "keptBuilds",
      "organization",
    ]);
  });

  it("compares nothing for a plan outside the catalog", () => {
    expect(planComparisonRows("legacy", "pro")).toEqual([]);
  });
});
