import { useMemo } from "react";
import { useWorkspaceRoster } from "@/common/hooks/use-workspace-roster";
import { normalizeEmail } from "@/features/avatars/lib/avatar-utils";

/**
 * The uploaded avatar of the current workspace's member with this email
 * (w1/m648), for the rows that name a person by email alone (a cron run's
 * "by"), as Render's `avatarsForWorkspaceMembers` looks them up; null when
 * nobody matches, the roster is not readable, or the member has no upload.
 */
export function useMemberAvatarURL(
  email: string | null | undefined,
): string | null {
  const key = normalizeEmail(email);
  const { members } = useWorkspaceRoster(!key);
  return useMemo(() => {
    if (!key) return null;
    const member = members.find(
      (candidate) => normalizeEmail(candidate?.email) === key,
    );
    return member?.avatarDownload?.url ?? null;
  }, [members, key]);
}
