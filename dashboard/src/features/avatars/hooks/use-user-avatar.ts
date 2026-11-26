import { useQuery } from "@apollo/client/react";
import { UserThemeSettingsDocument } from "@/graphql/definitions";
import { useHasVerifiedSession } from "@/common/hooks/use-has-verified-session";

/**
 * The signed-in account's id and uploaded avatar (w1/m648): Render's
 * `user { id avatarDownload { url } }`, read from the shell's own
 * UserThemeSettings query so the account is fetched once. Its User is the
 * object an upload or a reset writes into the cache, so the account menu
 * follows without a reload. Skipped without a session bex-api serves.
 */
export function useUserAvatar(): {
  userId: string | null;
  imageURL: string | null;
  loading: boolean;
} {
  const verified = useHasVerifiedSession();
  const { data, loading } = useQuery(UserThemeSettingsDocument, {
    skip: !verified,
    errorPolicy: "all",
  });
  return {
    userId: data?.user?.id ?? null,
    imageURL: data?.user?.avatarDownload?.url ?? null,
    loading: loading && data === undefined,
  };
}
