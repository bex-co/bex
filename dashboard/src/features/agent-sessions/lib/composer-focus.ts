// The sidebar's New-session keyboard shortcut and the composer agree on one
// window event (w3/m45 t004): the shortcut navigates to `/agents` and then
// dispatches this, and the mounted composer focuses its task textarea. An event
// (not a ref/context) because the two live in different route trees.

export const AGENT_COMPOSER_FOCUS_EVENT = "bex:agent-composer-focus";

/** Ask the mounted new-session composer (if any) to focus its prompt box. */
export function requestAgentComposerFocus(): void {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent(AGENT_COMPOSER_FOCUS_EVENT));
}
