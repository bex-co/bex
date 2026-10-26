import type { AgentSessionPhase } from "@/features/agent-sessions/types";

export type ConversationState =
  | "not-started"
  | "connecting"
  | "live"
  | "broken"
  | "ended";

export interface ConversationStateInput {
  phase?: AgentSessionPhase;
  isTerminal?: boolean;
  transportStatus?: string;
  hasMessages?: boolean;
  resuming?: boolean;
  transportError?: boolean;
}

const STARTING_PHASES = new Set<AgentSessionPhase>([
  "creating",
  "resuming",
  "redispatching",
]);

const TERMINAL_PHASES = new Set<AgentSessionPhase>([
  "completed",
  "failed",
  "canceled",
]);

/** One lifecycle+transport state machine shared by transcript and composer copy. */
export function deriveConversationState({
  phase,
  isTerminal = false,
  transportStatus,
  hasMessages = false,
  resuming = false,
  transportError = false,
}: ConversationStateInput): ConversationState {
  const transportBusy =
    resuming ||
    transportStatus === "submitted" ||
    transportStatus === "streaming";
  if (isTerminal || (phase && TERMINAL_PHASES.has(phase))) {
    // A terminal session still replaying its transcript is loading, not empty:
    // "ended" (and its "ended before a conversation was recorded" copy) waits
    // until the replay settled with nothing to show (w10/m16 t001).
    return !hasMessages && transportBusy && !transportError
      ? "connecting"
      : "ended";
  }
  if (!hasMessages && phase && STARTING_PHASES.has(phase)) {
    return transportBusy || transportError ? "connecting" : "not-started";
  }
  if (transportError) return "broken";
  if (!hasMessages && transportBusy) return "connecting";
  if (hasMessages || transportStatus === "ready") return "live";
  return "not-started";
}
