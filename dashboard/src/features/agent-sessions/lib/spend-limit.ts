// The per-session model spend ceiling (ADR094 D8, w21/m260): parsing the
// composer's "Spend limit (USD)" input, formatting the exact micro-dollar
// amounts `outcome.cost` carries, and telling a session-ceiling stop from the
// workspace's monthly cap (both fail `AGENT_MODEL_SPEND_CAP_REACHED`).

import type { AgentSessionView } from "@/features/agent-sessions/types";

/** Backend bounds: more than 0, at most 100000 (`AGENT_SESSION_SPEND_LIMIT_INVALID`). */
export const SPEND_LIMIT_MAX_USD = 100000;

export type SpendLimitParse =
  { ok: true; value: number | null } | { ok: false };

/** Blank means no ceiling; otherwise a finite number in (0, 100000]. */
export function parseSpendLimit(text: string): SpendLimitParse {
  const trimmed = text.trim();
  if (trimmed === "") return { ok: true, value: null };
  if (!/^\d*\.?\d+$|^\d+\.$/.test(trimmed)) return { ok: false };
  const value = Number(trimmed);
  if (!Number.isFinite(value) || value <= 0 || value > SPEND_LIMIT_MAX_USD) {
    return { ok: false };
  }
  return { ok: true, value };
}

/**
 * USD for a decimal-string amount, keeping the sub-cent digits the meter
 * records ("0.001234") but never fewer than two fractional digits.
 */
export function formatPreciseUsd(amount: string, locale: string): string {
  const value = Number(amount);
  if (!Number.isFinite(value)) return amount;
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
    maximumFractionDigits: 6,
  }).format(value);
}

/**
 * True when the session stopped at its own ceiling rather than the workspace's
 * monthly cap. The failure code is shared; the session's limit is set and its
 * recorded spend has reached it.
 */
export function stoppedAtSessionSpendLimit(
  session: Pick<AgentSessionView, "failureReasonCode" | "maxSpendUSD"> & {
    outcome: Pick<NonNullable<AgentSessionView["outcome"]>, "cost"> | null;
  },
): boolean {
  if (session.failureReasonCode !== "AGENT_MODEL_SPEND_CAP_REACHED") {
    return false;
  }
  if (!session.maxSpendUSD || !session.outcome?.cost) return false;
  return Number(session.outcome.cost.modelUSD) >= Number(session.maxSpendUSD);
}
