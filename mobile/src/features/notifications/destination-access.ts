import type { CapabilityAction } from "@/features/capabilities/capability-policy";

// Mirrors the backend's closed destination policy, using grants rather than
// role labels. A route must also be readable even for a malformed event/route
// pairing; parsing an envelope alone never grants destination access.
export function notificationActions(item: {
  event: string;
  route: string;
}): CapabilityAction[] {
  const actions: CapabilityAction[] = ["can_view"];
  if (
    item.route.startsWith("/sessions/") ||
    item.event === "agent_pr_ready" ||
    item.event === "agent_failed"
  )
    actions.push("can_operate");
  if (
    item.route.startsWith("/automations/") ||
    item.event === "agent_needs_decision" ||
    item.event === "agent_automation_paused"
  )
    actions.push("can_create");
  return actions;
}
