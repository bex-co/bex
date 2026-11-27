import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useAdminOnlyActions } from "@/features/capabilities/hooks/use-admin-only-actions";

export interface ConnectionRevealGate {
  /** The member may not read the datastore's connection info now. */
  disabled: boolean;
  /** Why, for the tooltip of the control it disables. */
  reason: string | undefined;
}

/**
 * Whether the caller may read a Postgres's or a Key Value's connection info,
 * whose strings carry the password: can_view_sensitive (w9/m84), and in a
 * protected environment an Admin's alone (w4/m594). One rule for the Info
 * card's Reveal and the header's Connect (w3/m675).
 */
export function useConnectionRevealGate(
  kind: "database" | "keyValue",
  id: string,
): ConnectionRevealGate {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const reasonKey = capabilities.reasonKey("can_view_sensitive");
  const adminOnly = useAdminOnlyActions(kind, id);
  const reason = reasonKey ? t(reasonKey) : adminOnly("view_connection_info");
  return { disabled: !capabilities.canViewSensitive || reason != null, reason };
}
