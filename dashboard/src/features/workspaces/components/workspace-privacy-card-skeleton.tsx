import type { ComponentProps } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { SettingRadioOption } from "@/features/workspaces/components/setting-radio-option";
import {
  WORKSPACE_PRIVACY_KEYS,
  WORKSPACE_PRIVACY_OPTIONS,
  type WorkspacePrivacy,
} from "@/features/workspaces/types";

/** The card's static header, shared by the ready card and its skeleton. */
export function WorkspacePrivacyCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("workspaces.privacyTitle")}</CardTitle>
      <CardDescription>{t("workspaces.privacyDescription")}</CardDescription>
    </CardHeader>
  );
}

/** One privacy setting as a card radio: Render's name and what it allows. */
export function WorkspacePrivacyOption({
  privacy,
  ...props
}: {
  privacy: WorkspacePrivacy;
} & Omit<ComponentProps<typeof SettingRadioOption>, "label" | "description">) {
  const { t } = useTranslations();
  const keys = WORKSPACE_PRIVACY_KEYS[privacy];
  return (
    <SettingRadioOption
      {...props}
      label={t(keys.label)}
      description={t(keys.description)}
    />
  );
}

/**
 * WorkspacePrivacyCard's pending shape (w7/m464): the static header, both
 * settings, and the Save button's own box (size sm). The card is an admin's
 * only, so there is no read-only variant to preview, and no control is
 * enabled before its data exists.
 */
export function WorkspacePrivacyCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="workspace-privacy-skeleton" aria-hidden>
      <WorkspacePrivacyCardHeader />
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 gap-2">
          {WORKSPACE_PRIVACY_OPTIONS.map((privacy) => (
            <WorkspacePrivacyOption
              key={privacy}
              privacy={privacy}
              checked={false}
              pending
              disabled
              tabIndex={-1}
            />
          ))}
        </div>
        <ReservedButton variant="default" size="sm">
          {t("workspaces.privacySave")}
        </ReservedButton>
      </CardContent>
    </Card>
  );
}
