// Pure agent-model spend logic (ADR091 D5, D6, D10.1): where a workspace's
// spend stands against the limits the mint holds it to, and what the
// composer should do about it. Settings → Agents and the new-session
// composer both read it, so the 80% / 100% thresholds live in one place.

import { AgentModelSource } from "@/graphql/definitions";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";

/** The share of the cap at which the composer starts warning (ADR091 D6). */
export const SPEND_WARNING_RATIO = 0.8;

/** The trust tier every new (or payment-locked) workspace starts at. */
export const NEW_WORKSPACE_TIER = "T0";

export type SpendLevel = "ok" | "warning" | "reached";

export interface SpendPosition {
  /** The workspace runs on the bex-provided model, so spend is capped. */
  platform: boolean;
  /**
   * The cap is the trust tier's charge point: reaching it charges the card,
   * and sessions keep running once that charge is paid (ADR091 D10.2a).
   */
  chargePoint: boolean;
  monthToDateUsd: number;
  /** The effective cap: min(the workspace's own cap, the tier's ceiling). */
  capUsd: number;
  /** The workspace's own monthly cap, which month-to-date is held against. */
  ownCapUsd: number;
  /**
   * The limit nearest to stopping new model requests, as the mint holds
   * each (spendcap.go RefusesAtCap): the trust tier's ceiling against the
   * spend billing credits and paid charges leave, or the workspace's own
   * cap against month-to-date. The meter and the warning show it.
   */
  limit: "ceiling" | "cap";
  /** The spend held against that limit, and the limit itself. */
  heldUsd: number;
  limitUsd: number;
  /**
   * The held spend is the ceiling's, and billing credits, paid charges or an
   * invoice that billed the meter make it differ from month-to-date, so the
   * copy names it rather than calling it this month's.
   */
  beyondCredits: boolean;
  /** heldUsd as a share of limitUsd, clamped to [0, 1] for display. */
  fraction: number;
  level: SpendLevel;
}

type Hold = Pick<SpendPosition, "limit" | "heldUsd" | "limitUsd">;

const share = ({ heldUsd, limitUsd }: Hold) =>
  limitUsd > 0 ? heldUsd / limitUsd : 1;

/**
 * The workspace's own monthly cap: the override clamped to the most an admin
 * may set, or the plan default. Where the tier's ceiling does not bind, it is
 * the effective cap. Null when the API sends neither.
 */
function ownCap(env: AgentEnvironmentView): number | null {
  if (!env.agentModelSpendCapTierLimited) {
    return env.agentModelMonthlySpendCapUsd;
  }
  const override = env.agentModelSpendCapOverrideUsd;
  const max = env.agentModelMaxSpendCapUsd;
  if (override == null) return env.agentModelPlanSpendCapUsd;
  return max == null ? override : Math.min(override, max);
}

/**
 * The workspace's spend position, or null when the environment carries no
 * spend fields (the store is off, or an older API).
 */
export function spendPosition(env: AgentEnvironmentView): SpendPosition | null {
  const used = env.agentModelSpendMonthToDateUsd;
  const cap = env.agentModelMonthlySpendCapUsd;
  if (used == null || cap == null) return null;
  const own = ownCap(env);
  // Where the tier's ceiling binds, the mint holds it against the spend
  // billing credits and paid charges leave (an older API sends none, so
  // month-to-date), whether or not bex can charge the workspace. The
  // workspace's own cap is held against month-to-date everywhere.
  const holds: Hold[] = [];
  if (env.agentModelSpendCapTierLimited) {
    holds.push({
      limit: "ceiling",
      heldUsd: env.agentModelUnbilledUsd ?? used,
      limitUsd: cap,
    });
  }
  if (own != null) holds.push({ limit: "cap", heldUsd: used, limitUsd: own });
  const nearest = holds.reduce((a, b) => (share(b) > share(a) ? b : a));
  const ratio = share(nearest);
  // The server's verdict is the mint's: a workspace whose next request would
  // charge its card is not reached. An older API sends none.
  const reached = env.agentModelSpendCapReached ?? ratio >= 1;
  return {
    platform: env.modelSource === AgentModelSource.Platform,
    chargePoint:
      env.agentModelSpendCapTierLimited === true &&
      !env.trustTierLocked &&
      env.agentModelChargesAtThreshold === true,
    monthToDateUsd: used,
    capUsd: cap,
    ownCapUsd: own ?? cap,
    ...nearest,
    beyondCredits: nearest.limit === "ceiling" && nearest.heldUsd !== used,
    fraction: Math.min(1, Math.max(0, ratio)),
    level: reached
      ? "reached"
      : ratio >= SPEND_WARNING_RATIO
        ? "warning"
        : "ok",
  };
}

/** Why the cap binds, for the copy that explains a reached cap. */
export type CapCause = "cap" | "newWorkspace" | "locked";

export function capCause(env: AgentEnvironmentView): CapCause {
  if (env.trustTierLocked) return "locked";
  // The mint names the workspace's own cap once month-to-date reaches it,
  // even while the tier's ceiling sets the effective cap
  // (SpendCapReachedReason).
  const own = ownCap(env);
  const used = env.agentModelSpendMonthToDateUsd;
  const ownReached = own != null && used != null && used >= own;
  if (env.agentModelSpendCapTierLimited && !ownReached) return "newWorkspace";
  return "cap";
}

/**
 * What the new-session composer shows for the workspace's model spend. Only
 * a platform-route workspace is capped: a BYO-key workspace's provider bills
 * it directly, so it never sees spend UI.
 */
export type ComposerSpendGate =
  | { kind: "none" }
  | { kind: "warning"; position: SpendPosition }
  | {
      kind: "reached";
      position: SpendPosition;
      cause: CapCause;
      /**
       * The workspace's own cap binds and its override can still rise (below
       * agentModelMaxSpendCapUsd), so an admin can lift it in Settings.
       */
      canRaiseCap: boolean;
    }
  | { kind: "balanceDue" };

export function composerSpendGate(
  env: AgentEnvironmentView | null,
): ComposerSpendGate {
  if (!env || env.modelSource !== AgentModelSource.Platform) {
    return { kind: "none" };
  }
  if (env.billingBalanceDue) return { kind: "balanceDue" };
  const position = spendPosition(env);
  if (!position) return { kind: "none" };
  if (position.level === "reached") {
    const cause = capCause(env);
    return {
      kind: "reached",
      position,
      cause,
      canRaiseCap: cause === "cap" && capCanRise(env),
    };
  }
  if (position.level === "warning") return { kind: "warning", position };
  return { kind: "none" };
}

/**
 * Whether the workspace's cap override (or, without one, the plan default it
 * runs on) is below the most an admin may set.
 */
export function capCanRise(env: AgentEnvironmentView): boolean {
  const max = env.agentModelMaxSpendCapUsd;
  const current =
    env.agentModelSpendCapOverrideUsd ?? env.agentModelPlanSpendCapUsd;
  return max != null && current != null && current < max;
}

/** Whether the gate refuses new sessions (the mint would refuse them). */
export function blocksCreate(gate: ComposerSpendGate): boolean {
  return gate.kind === "reached" || gate.kind === "balanceDue";
}

/**
 * A cap override as typed: a positive amount at most `maxUsd`, to the cent.
 * Null when the server would refuse it with AGENT_ENVIRONMENT_SPEND_CAP_INVALID.
 */
export function parseSpendCap(
  text: string,
  maxUsd: number | null,
): number | null {
  const trimmed = text.trim();
  if (!/^\d+(\.\d{1,2})?$/.test(trimmed)) return null;
  const value = Number(trimmed);
  if (!Number.isFinite(value) || value <= 0) return null;
  if (maxUsd != null && value > maxUsd) return null;
  return value;
}
