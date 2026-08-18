import type { Capabilities } from "@/features/capabilities/hooks/use-capabilities";

/**
 * A fully-permitted capability set (every relation granted, role ADMIN) for
 * tests that render a role-gated control (w9/m84). Pass overrides to model a
 * lower role — e.g. `mockCapabilities({ role: "CONTRIBUTOR", canCreate: false })`.
 */
export function mockCapabilities(
  overrides: Partial<Capabilities> = {},
): Capabilities {
  return {
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
    ...overrides,
  };
}
