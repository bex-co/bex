/**
 * The caller's effective permissions in the active workspace (w9/m84, refreshed
 * w6/m144). Fail-closed: unknown/error/stale never grants actions or sensitive
 * reveals. Mounted consumers share one CapabilitiesProvider refresh lifecycle.
 */
export type { Capabilities } from "@/features/capabilities/context/capabilities-provider";
export { useCapabilitiesContext as useCapabilities } from "@/features/capabilities/context/capabilities-provider";
export type { CapabilityAction } from "@/features/capabilities/lib/capability-policy";
export {
  actionForBoolean,
  grantReasonKey,
} from "@/features/capabilities/lib/capability-policy";
