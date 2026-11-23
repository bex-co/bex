// Who asks a datastore's or an environment group's admin-only controls
// (w4/m594), as useAdminOnlyActions answers each: an Admin (nothing refused), a
// Developer in a protected environment (Render's admin-only reason), or a
// Viewer (the role reason). The hook itself is mocked in test/setup.ts.
import { vi } from "vitest";
import { useAdminOnlyActions } from "@/features/capabilities/hooks/use-admin-only-actions";

export type AdminOnlyCaller = "admin" | "developer" | "viewer";

export const ADMIN_ONLY_REASON = "This action is limited to admins.";
export const ROLE_REASON =
  "Your role can’t perform this action. Ask a workspace admin.";

const REASONS: Record<AdminOnlyCaller, string | undefined> = {
  admin: undefined,
  developer: ADMIN_ONLY_REASON,
  viewer: ROLE_REASON,
};

/** Makes every useAdminOnlyActions call answer `caller`'s reason. */
export function adminOnlyAs(caller: AdminOnlyCaller) {
  vi.mocked(useAdminOnlyActions).mockReturnValue(() => REASONS[caller]);
}
