// The model a session runs (w10/m516): display helpers shared by the model
// picker and the session header.

import type { AgentModelOffer } from "@/features/agent-sessions/types";

/**
 * A model id without its provider prefix ("anthropic/claude-opus-5.5" →
 * "claude-opus-5.5"), for a control too narrow for the whole id. The full id
 * stays in the control's title and its list.
 */
export function shortModel(id: string): string {
  return id.slice(id.lastIndexOf("/") + 1).replace(/^~/, "");
}

/**
 * The model a picker shows: the chosen one, else the one the session or the
 * profile runs now.
 */
export function shownModel(
  offer: AgentModelOffer,
  value: string,
  current?: string | null,
): string {
  return value || current || offer.model || offer.models[0] || "";
}

/**
 * Whether a saved model is one the agent's catalog no longer offers
 * (w10/m527): an admin turned it off in Settings → Agents, or it left the
 * platform catalog. False with nothing saved, and where the offer lists no
 * models (the workspace's own key takes any model id).
 */
export function modelUnoffered(offer: AgentModelOffer, model: string): boolean {
  return (
    model !== "" && offer.models.length > 0 && !offer.models.includes(model)
  );
}
