// Pure agent-model spend logic (ADR091 D5, D6, D10.1): where a workspace's
// month-to-date spend stands against its effective cap, and what the
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
  capUsd: number;
  /** Month-to-date as a share of the cap, clamped to [0, 1] for display. */
  fraction: number;
  level: SpendLevel;
}

/**
 * The workspace's spend position, or null when the environment carries no
 * spend fields (the store is off, or an older API).
 */
export function spendPosition(env: AgentEnvironmentView): SpendPosition | null {
  const used = env.agentModelSpendMonthToDateUsd;
  const cap = env.agentModelMonthlySpendCapUsd;
  if (used == null || cap == null) return null;
  const ratio = cap > 0 ? used / cap : 1;
  // The server's verdict is the mint's: a workspace whose next request would
  // charge its card is not reached. An older API sends none.
  const reached = env.agentModelSpendCapReached ?? ratio >= 1;
  const chargePoint =
    capCause(env) === "newWorkspace" &&
    env.agentModelChargesAtThreshold === true;
  // At a charge point the card is charged on the spend billing credits leave
  // (ADR091 D10.2a), so that, not month-to-date, is what nears it.
  const unbilled = env.agentModelUnbilledUsd;
  const nearing =
    chargePoint && unbilled != null && cap > 0 ? unbilled / cap : ratio;
  return {
    platform: env.modelSource === AgentModelSource.Platform,
    chargePoint,
    monthToDateUsd: used,
    capUsd: cap,
    fraction: Math.min(1, Math.max(0, ratio)),
    level: reached
      ? "reached"
      : nearing >= SPEND_WARNING_RATIO
        ? "warning"
        : "ok",
  };
}

/** Why the cap binds, for the copy that explains a reached cap. */
export type CapCause = "cap" | "newWorkspace" | "locked";

export function capCause(env: AgentEnvironmentView): CapCause {
  if (env.trustTierLocked) return "locked";
  if (env.agentModelSpendCapTierLimited) return "newWorkspace";
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
