import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { ChangeWorkspaceMemberRoleDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { Role } from "@/features/team/types";

export interface UseChangeRoleResult {
  /** Fires changeWorkspaceMemberRole; resolves true on success. */
  changeRole: (subject: string, role: Role) => Promise<boolean>;
  /** The subject currently being changed, or null (disables that row's control). */
  changing: string | null;
}

/**
 * Wires a member row's role dropdown to bex-api's `changeWorkspaceMemberRole`.
 * The last-admin refusal is enforced server-side (docs/ADR012-auth.md): demoting the
 * only admin returns a bad-request the UI surfaces as a toast.
 */
export function useChangeRole(workspaceId: string): UseChangeRoleResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(ChangeWorkspaceMemberRoleDocument);
  const [changing, setChanging] = useState<string | null>(null);

  const changeRole = useCallback(
    async (subject: string, role: Role) => {
      setChanging(subject);
      try {
        await mutate({ variables: { workspaceId, subject, role } });
        toast.success(t("team.roleChanged"));
        return true;
      } catch (e) {
        const msg = e instanceof Error ? e.message : "";
        toast.error(
          msg.toLowerCase().includes("last admin")
            ? t("team.lastAdminError")
            : t("team.roleChangeError"),
        );
        return false;
      } finally {
        setChanging(null);
      }
    },
    [mutate, workspaceId, t],
  );

  return { changeRole, changing };
}
