import { useCallback } from "react";
import { useApolloClient } from "@apollo/client/react";
import {
  AvatarPresignedUploadDocument,
  DeleteAvatarDocument,
  UserThemeSettingsDocument,
  WorkspacesDocument,
} from "@/graphql/definitions";
import { AvatarType } from "@/graphql/schema-types";
import { readAsDataURL } from "@/features/avatars/lib/avatar-image";
import { resolveAvatarUrl } from "@/features/avatars/lib/avatar-utils";

export { AvatarType };

/** The cached objects an owner's avatarDownload lives on, and the query that
 * reads it for the shell (the account menu, the switcher), refetched after a
 * change for the image's own URL. */
const OWNER = {
  [AvatarType.Account]: {
    typenames: ["User"],
    refetch: UserThemeSettingsDocument,
  },
  [AvatarType.Workspace]: {
    typenames: ["Workspace", "OrganizationWorkspace"],
    refetch: WorkspacesDocument,
  },
} as const;

/**
 * Render's two avatar operations (w1/m648): `upload` asks
 * avatarPresignedUpload for a one-time URL and PUTs the JPEG to it; `reset`
 * is deleteAvatar. Each puts the result in Apollo's cache at once (the
 * uploaded blob's data URL, or null), so the account menu, the switcher and
 * the row change without a reload, then refetches the real URL. Each throws
 * when the server refuses, for the caller's toast.
 */
export function useAvatarActions(ownerId: string, avatarType: AvatarType) {
  const client = useApolloClient();

  const show = useCallback(
    (url: string | null) => {
      const owner = OWNER[avatarType];
      for (const __typename of owner.typenames) {
        const id = client.cache.identify({ __typename, id: ownerId });
        if (!id) continue;
        client.cache.modify({
          id,
          fields: {
            avatarDownload: () =>
              url ? { __typename: "PresignedDownload", url } : null,
          },
        });
      }
      void client
        .refetchQueries({ include: [owner.refetch] })
        .catch(() => undefined);
    },
    [client, ownerId, avatarType],
  );

  const upload = useCallback(
    async (image: Blob) => {
      const { data } = await client.query({
        query: AvatarPresignedUploadDocument,
        variables: {
          avatarOwnerId: ownerId,
          avatarType,
          avatarSizeBytes: image.size,
        },
        fetchPolicy: "no-cache",
      });
      const target = data?.avatarPresignedUpload;
      const url = resolveAvatarUrl(target?.url);
      if (!target || !url)
        throw new Error("No presigned upload info provided.");
      const response = await fetch(url, {
        method: target.verb,
        body: image,
        headers: { "Content-Type": image.type || "image/jpeg" },
      });
      if (!response.ok) {
        throw new Error(`avatar upload answered ${response.status}`);
      }
      show(await readAsDataURL(image));
    },
    [client, ownerId, avatarType, show],
  );

  const reset = useCallback(async () => {
    await client.mutate({
      mutation: DeleteAvatarDocument,
      variables: { avatarOwnerId: ownerId, avatarType },
    });
    show(null);
  }, [client, ownerId, avatarType, show]);

  return { upload, reset };
}
