import type { Capabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  BOOLEAN_TO_ACTION,
  ROLE_REASON_KEYS,
  type CapabilityAction,
  type CapabilityBoolean,
} from "@/features/capabilities/lib/capability-policy";
import enCapabilities from "@/features/capabilities/locales/en";

/** The en `capabilities.reasonCanCreate` sentence a can_create-gated control shows. */
export const CREATE_REASON =
  "Your role can’t make this change. Ask a workspace admin.";

/** Each action's boolean on `Capabilities`, the policy's map inverted. */
const BOOLEAN_FOR_ACTION = Object.fromEntries(
  Object.entries(BOOLEAN_TO_ACTION).map(([key, action]) => [action, key]),
) as Record<CapabilityAction, CapabilityBoolean>;

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
    canViewBilling: true,
    canViewActivity: true,
    loading: false,
    loaded: true,
    stale: false,
    unavailable: false,
    generation: 1,
    checkedAt: 1,
    allows: () => true,
    denied: () => false,
    reasonKey: () => undefined,
    refresh: async () => undefined,
  };
  const merged = { ...base, ...overrides };
  // Keep helpers consistent with boolean overrides when tests only flip can*.
  if (!overrides.allows) {
    merged.allows = (action) => merged[BOOLEAN_FOR_ACTION[action]];
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
      return ROLE_REASON_KEYS[action];
    };
  }
  return merged;
}

/** The en `capabilities.reasonCanViewActivity` sentence (w9/m621). */
export const ACTIVITY_REASON =
  enCapabilities["capabilities.reasonCanViewActivity"].message;

/** The en `capabilities.reasonCanViewLogs` sentence (w9/145). */
export const LOGS_REASON =
  enCapabilities["capabilities.reasonCanViewLogs"].message;

/**
 * A Billing member's confirmed grants, as on Render (w9/m620, w9/m621): it
 * reads the workspace's billing, and no service events, deploys, metrics or
 * logs, and changes nothing else.
 */
export function billingCapabilities(): Capabilities {
  return mockCapabilities({
    role: "BILLING",
    canViewLogs: false,
    canOperate: false,
    canCreate: false,
    canViewSensitive: false,
    canManageKeys: false,
    canManage: false,
    canViewActivity: false,
  });
}

/**
 * A Viewer's confirmed grants (w9/m621): it reads service events, deploys and
 * metrics, but no logs or billing, and changes nothing.
 */
export function viewerCapabilities(): Capabilities {
  return mockCapabilities({
    role: "VIEWER",
    canViewLogs: false,
    canOperate: false,
    canCreate: false,
    canViewSensitive: false,
    canManageKeys: false,
    canManage: false,
    canManageBilling: false,
    canViewBilling: false,
  });
}

/**
 * A Contributor's confirmed grants (w9/m619, w9/145): it reads service
 * activity and logs and runs the runtime actions, but changes no setting and
 * reads no secret or billing.
 */
export function contributorCapabilities(): Capabilities {
  return mockCapabilities({
    role: "CONTRIBUTOR",
    canCreate: false,
    canViewSensitive: false,
    canManageKeys: false,
    canManage: false,
    canManageBilling: false,
    canViewBilling: false,
  });
}
