import { Card } from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { AvatarEditorSkeleton } from "@/features/avatars/components/avatar-editor-skeleton";
import { APPEARANCE_CARD_CLASS, AppearanceRow } from "./appearance-row";

/**
 * The Avatar card while the account loads (w1/m648): the ready row's label
 * beside the 64 px circle over the Edit button's box. Skeleton-only, so the
 * route's pending component keeps it out of the page's chunk.
 */
export function AccountAvatarCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card
      aria-hidden="true"
      data-testid="account-avatar-card-skeleton"
      className={APPEARANCE_CARD_CLASS}
    >
      <AppearanceRow label={t("avatars.rowLabel")}>
        <AvatarEditorSkeleton />
      </AppearanceRow>
    </Card>
  );
}
