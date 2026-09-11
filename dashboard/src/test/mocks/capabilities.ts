import type { Capabilities } from "@/features/capabilities/hooks/use-capabilities";
import type { CapabilityAction } from "@/features/capabilities/lib/capability-policy";

/**
 * A fully-permitted capability set (every relation granted, role ADMIN) for
 * tests that render a role-gated control (w9/m84 / w6/m144). Pass overrides to
 * model a lower role — e.g. `mockCapabilities({ role: "CONTRIBUTOR", canCreate: false })`.
 */
export function mockCapabilities(
  overrides: Partial<Capabilities> = {},
): Capabilities {
  const base: Capabilities = {
    role: "ADMIN",
    canView: true,
    canViewLogs: true,
    canOperate: true,
    canCreate: true,
    canViewSensitive: true,
    canManageKeys: true,
    canManage: true,
    canManageBilling: true,
    loading: false,
    loaded: true,
    stale: false,
    unavailable: false,
    generation: 1,
    allows: (action: CapabilityAction) => {
      const map: Record<CapabilityAction, boolean> = {
        can_view: true,
        can_view_logs: true,
        can_operate: true,
        can_create: true,
        can_view_sensitive: true,
        can_manage_keys: true,
        can_manage: true,
        can_manage_billing: true,
      };
      return map[action] ?? false;
    },
    denied: () => false,
    reasonKey: () => undefined,
    refresh: async () => undefined,
  };
  const merged = { ...base, ...overrides };
  // Keep helpers consistent with boolean overrides when tests only flip can*.
  if (!overrides.allows) {
    merged.allows = (action) => {
      const byAction: Record<CapabilityAction, keyof Capabilities> = {
        can_view: "canView",
        can_view_logs: "canViewLogs",
        can_operate: "canOperate",
        can_create: "canCreate",
        can_view_sensitive: "canViewSensitive",
        can_manage_keys: "canManageKeys",
        can_manage: "canManage",
        can_manage_billing: "canManageBilling",
      };
      return Boolean(merged[byAction[action]]);
    };
  }
  if (!overrides.denied) {
    merged.denied = (action) =>
      Boolean(merged.loaded) && !merged.allows(action);
  }
  if (!overrides.reasonKey) {
    merged.reasonKey = (action) => {
      if (merged.allows(action)) return undefined;
      if (!merged.loaded) {
        if (merged.stale) return "capabilities.grantStale";
        if (merged.unavailable) return "capabilities.grantUnavailable";
        return undefined;
      }
      const keys: Record<CapabilityAction, string> = {
        can_view: "capabilities.reasonCanCreate",
        can_view_logs: "capabilities.reasonCanOperate",
        can_operate: "capabilities.reasonCanOperate",
        can_create: "capabilities.reasonCanCreate",
        can_view_sensitive: "capabilities.reasonCanViewSensitive",
        can_manage_keys: "capabilities.reasonCanCreate",
        can_manage: "capabilities.reasonCanManage",
        can_manage_billing: "capabilities.reasonCanManageBilling",
      };
      return keys[action];
    };
  }
  return merged;
}
