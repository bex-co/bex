import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  RemoveWorkspaceMemberDocument,
  RevokeWorkspaceInviteDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseRemoveMemberResult {
  /** Removes an accepted member; resolves true on success. */
  removeMember: (subject: string) => Promise<boolean>;
  /** Revokes a pending invite; resolves true on success. */
  revokeInvite: (inviteId: string) => Promise<boolean>;
  /** The subject/invite currently being removed, or null. */
  removing: string | null;
}

/**
 * Wires the destructive team actions to bex-api: removing an accepted member
 * (`removeWorkspaceMember`) and revoking a pending invite
 * (`revokeWorkspaceInvite`). Removing the last admin is refused server-side and
 * surfaced as a toast. No optimistic update — a failed remove leaves the row.
 */
export function useRemoveMember(workspaceId: string): UseRemoveMemberResult {
  const { t } = useTranslations();
  const [removeMut] = useMutation(RemoveWorkspaceMemberDocument);
  const [revokeMut] = useMutation(RevokeWorkspaceInviteDocument);
  const [removing, setRemoving] = useState<string | null>(null);

  const removeMember = useCallback(
    async (subject: string) => {
      setRemoving(subject);
      try {
        await removeMut({ variables: { workspaceId, subject } });
        toast.success(t("team.removeSuccess"));
        return true;
      } catch (e) {
        const msg = e instanceof Error ? e.message : "";
        toast.error(
          msg.toLowerCase().includes("last admin")
            ? t("team.lastAdminError")
            : t("team.removeError"),
        );
        return false;
      } finally {
        setRemoving(null);
      }
    },
    [removeMut, workspaceId, t],
  );

  const revokeInvite = useCallback(
    async (inviteId: string) => {
      setRemoving(inviteId);
      try {
        await revokeMut({ variables: { workspaceId, inviteId } });
        toast.success(t("team.revokeInviteSuccess"));
        return true;
      } catch {
        toast.error(t("team.revokeInviteError"));
        return false;
      } finally {
        setRemoving(null);
      }
    },
    [revokeMut, workspaceId, t],
  );

  return { removeMember, revokeInvite, removing };
}
