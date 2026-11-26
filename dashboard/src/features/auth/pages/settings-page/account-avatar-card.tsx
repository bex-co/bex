import { Card } from "@/common/components/ui/card";
import { useRootContext } from "@/common/hooks/use-root-context";
import { useTranslations } from "@/common/hooks/use-translations";
import { UserAvatar } from "@/features/avatars/components/avatar";
import { AvatarEditor } from "@/features/avatars/components/avatar-row";
import { AvatarType } from "@/features/avatars/hooks/use-avatar-actions";
import { useUserAvatar } from "@/features/avatars/hooks/use-user-avatar";
import { emailMonogram } from "@/features/avatars/lib/avatar-utils";
import { AccountAvatarCardSkeleton } from "./account-avatar-card-skeleton";
import { APPEARANCE_CARD_CLASS, AppearanceRow } from "./appearance-row";

type IdentityTraits = { email?: string };

/**
 * Account Settings' Avatar, after the profile as on Render (w1/m648,
 * docs/render-artifacts/avatars.md): the account's avatar (its upload, else
 * its Gravatar, else the monogram of its email) with Render's Edit menu. Only
 * the account itself may change it, so the menu is always enabled here.
 */
export function AccountAvatarCard() {
  const { t } = useTranslations();
  const { session } = useRootContext();
  const email = (session?.identity?.traits as IdentityTraits | undefined)
    ?.email;
  const { userId, imageURL, loading } = useUserAvatar();
  if (loading) return <AccountAvatarCardSkeleton />;
  if (!userId) return null;
  return (
    <Card data-testid="account-avatar-card" className={APPEARANCE_CARD_CLASS}>
      <AppearanceRow label={t("avatars.rowLabel")}>
        <AvatarEditor
          ownerId={userId}
          avatarType={AvatarType.Account}
          avatar={
            <UserAvatar email={email} imageURL={imageURL} size="xlarge" />
          }
          isUpload={imageURL !== null}
          defaultMonogram={emailMonogram(email)}
          canEdit
        />
      </AppearanceRow>
    </Card>
  );
}
