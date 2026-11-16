import { describe, expect, it } from "vitest";
import { AgentModelSource } from "@/graphql/definitions";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import {
  blocksCreate,
  composerSpendGate,
  parseSpendCap,
  spendPosition,
} from "@/features/agent-environment/lib/spend";

const env = (used: number | null, cap: number | null, over = {}) =>
  agentEnvironment({
    agentModelSpendMonthToDateUsd: used,
    agentModelMonthlySpendCapUsd: cap,
    ...over,
  });

describe("spendPosition", () => {
  it("places spend against the cap at the 80% and 100% thresholds", () => {
    expect(spendPosition(env(15.99, 20))?.level).toBe("ok");
    expect(spendPosition(env(16, 20))?.level).toBe("warning");
    expect(spendPosition(env(20, 20))?.level).toBe("reached");
    expect(spendPosition(env(30, 20))).toMatchObject({
      level: "reached",
      fraction: 1,
    });
    expect(spendPosition(env(null, 20))).toBeNull();
    // A zero cap is already reached, even before any spend.
    expect(spendPosition(env(0, 0))).toMatchObject({
      level: "reached",
      fraction: 1,
    });
  });
});

describe("the server's cap verdict", () => {
  it("is the mint's: a T0 workspace whose next request charges its card is not reached", () => {
    const charging = env(10.5, 10, {
      agentModelSpendCapTierLimited: true,
      agentModelSpendCapReached: false,
      agentModelChargesAtThreshold: true,
    });
    expect(spendPosition(charging)).toMatchObject({
      level: "warning",
      chargePoint: true,
      fraction: 1,
    });
    expect(blocksCreate(composerSpendGate(charging))).toBe(false);
    // Reached when the server says so, whatever the figures read.
    expect(
      composerSpendGate(
        env(10, 10, {
          agentModelSpendCapTierLimited: true,
          agentModelSpendCapReached: true,
        }),
      ),
    ).toMatchObject({ kind: "reached", cause: "newWorkspace" });
    // A workspace bex cannot charge, or one under a payment hold, has a
    // ceiling, never a charge point.
    expect(
      spendPosition(
        env(9, 10, {
          agentModelSpendCapTierLimited: true,
          agentModelChargesAtThreshold: false,
        }),
      )?.chargePoint,
    ).toBe(false);
    expect(
      spendPosition(
        env(9, 10, {
          agentModelSpendCapTierLimited: true,
          trustTierLocked: true,
        }),
      )?.chargePoint,
    ).toBe(false);
    expect(spendPosition(env(16, 20))?.chargePoint).toBe(false);
  });
});

describe("billing credits at a charge point (w6/m400)", () => {
  const t0 = (used: number, unbilled: number | null) =>
    env(used, 10, {
      agentModelSpendCapTierLimited: true,
      agentModelSpendCapReached: false,
      agentModelChargesAtThreshold: true,
      agentModelUnbilledUsd: unbilled,
    });
  it("warns on the spend the credits leave, not month-to-date", () => {
    // $9 spent, $7 of it covered by credits: $2 toward the $10 charge point.
    expect(spendPosition(t0(9, 2))?.level).toBe("ok");
    expect(spendPosition(t0(15, 8))?.level).toBe("warning");
    // An older API without the unbilled figure falls back to month-to-date.
    expect(spendPosition(t0(9, null))?.level).toBe("warning");
  });
  it("keeps month-to-date for a cap that is the workspace's own", () => {
    expect(
      spendPosition(env(16, 20, { agentModelUnbilledUsd: 1 }))?.level,
    ).toBe("warning");
  });
});

describe("the tier's ceiling on a workspace bex cannot charge (w6/131)", () => {
  // bex-canary on production: T0, billing-excluded, credits far above its
  // spend. The mint holds the $10 ceiling against the unbilled spend and the
  // workspace's own $20 cap against month-to-date.
  const ceiling = (used: number, unbilled: number, over = {}) =>
    env(used, 10, {
      agentModelSpendCapTierLimited: true,
      agentModelSpendCapReached: false,
      agentModelChargesAtThreshold: false,
      agentModelPlanSpendCapUsd: 20,
      agentModelMaxSpendCapUsd: 200,
      agentModelCreditUsd: 1_000_000,
      agentModelUnbilledUsd: unbilled,
      ...over,
    });
  const locked = (used: number, unbilled: number) =>
    ceiling(used, unbilled, { trustTierLocked: true });

  it.each([
    ["an excluded workspace", ceiling],
    ["a payment-locked workspace", locked],
  ])("does not warn %s whose credits cover month-to-date", (_, at) => {
    const position = spendPosition(at(9, 0));
    expect(position).toMatchObject({
      level: "ok",
      chargePoint: false,
      limit: "cap",
      heldUsd: 9,
      limitUsd: 20,
    });
    expect(position?.fraction).toBeCloseTo(0.45);
    expect(composerSpendGate(at(9, 0)).kind).toBe("none");
  });

  it.each([
    ["an excluded workspace", ceiling],
    ["a payment-locked workspace", locked],
  ])("warns %s whose unbilled spend nears the ceiling", (_, at) => {
    // $15 this month, $8 of it beyond the credits: 80% of the $10 ceiling.
    expect(spendPosition(at(15, 8))).toMatchObject({
      level: "warning",
      chargePoint: false,
      limit: "ceiling",
      heldUsd: 8,
      limitUsd: 10,
      fraction: 0.8,
      beyondCredits: true,
    });
    expect(composerSpendGate(at(15, 8)).kind).toBe("warning");
    expect(spendPosition(at(15, 7.99))?.level).toBe("ok");
  });

  it("warns on the workspace's own cap as month-to-date nears it", () => {
    expect(spendPosition(ceiling(16, 0))).toMatchObject({
      level: "warning",
      limit: "cap",
      heldUsd: 16,
      limitUsd: 20,
      ownCapUsd: 20,
      fraction: 0.8,
      beyondCredits: false,
    });
    // An override past the most an admin may set binds at that most.
    expect(
      spendPosition(
        ceiling(160, 0, {
          agentModelSpendCapOverrideUsd: 500,
          agentModelMaxSpendCapUsd: 200,
        }),
      ),
    ).toMatchObject({ level: "warning", limitUsd: 200 });
  });

  it("names the workspace's own cap once month-to-date reaches it", () => {
    const gate = composerSpendGate(
      ceiling(20, 0, { agentModelSpendCapReached: true }),
    );
    expect(gate).toMatchObject({
      kind: "reached",
      cause: "cap",
      canRaiseCap: true,
      position: { ownCapUsd: 20, capUsd: 10 },
    });
    // The ceiling, not the cap, refuses a workspace short of its own cap.
    expect(
      composerSpendGate(ceiling(12, 10, { agentModelSpendCapReached: true })),
    ).toMatchObject({ kind: "reached", cause: "newWorkspace" });
    // A payment hold is named first, as the mint names it.
    expect(
      composerSpendGate(
        ceiling(20, 0, {
          trustTierLocked: true,
          agentModelSpendCapReached: true,
        }),
      ),
    ).toMatchObject({ kind: "reached", cause: "locked" });
  });

  it("calls the ceiling's spend this month's when no credit moves it", () => {
    expect(
      spendPosition(ceiling(8.5, 8.5, { agentModelCreditUsd: 0 })),
    ).toMatchObject({ limit: "ceiling", beyondCredits: false });
  });
});

describe("composerSpendGate", () => {
  it("gates only the platform route", () => {
    expect(composerSpendGate(null).kind).toBe("none");
    expect(
      composerSpendGate(
        env(50, 20, {
          modelSource: AgentModelSource.Byo,
          billingBalanceDue: true,
        }),
      ).kind,
    ).toBe("none");
    expect(
      composerSpendGate(env(50, 20, { modelSource: AgentModelSource.None }))
        .kind,
    ).toBe("none");
  });

  it("puts a balance due before the cap, and names the cap's cause", () => {
    const due = composerSpendGate(env(1, 20, { billingBalanceDue: true }));
    expect(due.kind).toBe("balanceDue");
    expect(blocksCreate(due)).toBe(true);
    expect(composerSpendGate(env(20, 20))).toMatchObject({
      kind: "reached",
      cause: "cap",
    });
    expect(
      composerSpendGate(env(10, 10, { agentModelSpendCapTierLimited: true })),
    ).toMatchObject({ kind: "reached", cause: "newWorkspace" });
    expect(
      composerSpendGate(env(10, 10, { trustTierLocked: true })),
    ).toMatchObject({ kind: "reached", cause: "locked" });
    const warning = composerSpendGate(env(16, 20));
    expect(warning.kind).toBe("warning");
    expect(blocksCreate(warning)).toBe(false);
  });
});

describe("canRaiseCap", () => {
  const capped = (over = {}) =>
    composerSpendGate(
      env(20, 20, {
        agentModelPlanSpendCapUsd: 20,
        agentModelMaxSpendCapUsd: 200,
        ...over,
      }),
    );

  it("offers a raise only while the workspace's own cap binds below the max", () => {
    expect(capped()).toMatchObject({ cause: "cap", canRaiseCap: true });
    expect(capped({ agentModelSpendCapOverrideUsd: 150 })).toMatchObject({
      canRaiseCap: true,
    });
    // The override already sits at the maximum: nothing left to raise.
    expect(capped({ agentModelSpendCapOverrideUsd: 200 })).toMatchObject({
      cause: "cap",
      canRaiseCap: false,
    });
    // No maximum read: never offer it.
    expect(capped({ agentModelMaxSpendCapUsd: null })).toMatchObject({
      canRaiseCap: false,
    });
    // The tier's ceiling or a payment hold binds, not the cap.
    expect(
      capped({
        agentModelSpendCapTierLimited: true,
        agentModelMonthlySpendCapUsd: 10,
        agentModelSpendMonthToDateUsd: 10,
      }),
    ).toMatchObject({ cause: "newWorkspace", canRaiseCap: false });
    expect(capped({ trustTierLocked: true })).toMatchObject({
      cause: "locked",
      canRaiseCap: false,
    });
  });
});

describe("parseSpendCap", () => {
  it("accepts a positive amount to the cent within the bound", () => {
    expect(parseSpendCap(" 50 ", 200)).toBe(50);
    expect(parseSpendCap("12.34", 200)).toBe(12.34);
    expect(parseSpendCap("200", 200)).toBe(200);
    expect(parseSpendCap("200.01", 200)).toBeNull();
    expect(parseSpendCap("0", 200)).toBeNull();
    expect(parseSpendCap("1.234", 200)).toBeNull();
    expect(parseSpendCap("-5", 200)).toBeNull();
    expect(parseSpendCap("", 200)).toBeNull();
  });
});
