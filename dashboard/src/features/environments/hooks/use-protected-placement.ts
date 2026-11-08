import { useCallback } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { protectedEnvironment } from "@/features/environments/lib/placement";

/**
 * Render's protected-environment placement rule as the dashboard shows it
 * (w3/m281): only an Admin or Owner — the workspace's `can_manage`, the same
 * fresh relation bex-api checks — may create a resource in a protected
 * environment or move one into or out of it. `restricts(e)` is true only for
 * a confirmed non-Admin, so an unloaded capability answer never disables a
 * control the server would allow; the server refuses a raced attempt with the
 * same `reason` (`placementErrorMessage`).
 */
export function useProtectedPlacement() {
  const { t } = useTranslations();
  const { loaded, canManage } = useCapabilities();
  const restricted = loaded && !canManage;
  const restricts = useCallback(
    (environment: { protectedStatus?: string } | null | undefined) =>
      restricted && protectedEnvironment(environment),
    [restricted],
  );
  return {
    restricts,
    reason: t("environments.protectedPlacementAdminOnly"),
  };
}
