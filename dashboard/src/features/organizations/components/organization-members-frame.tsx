import type { ReactNode } from "react";
import { AlertTriangle, Lock } from "lucide-react";
import {
  CardAction,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { PanelCenteredState } from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";

/** Every member row holds a small control's height, with or without one. */
export const MEMBER_ROW_CLASS = "h-12";

/** The Members card's static header, with the Owner's invite action. */
export function OrganizationMembersCardHeader({
  action,
}: {
  action?: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("organizations.membersCardTitle")}</CardTitle>
      <CardDescription>
        {t("organizations.membersCardDescription")}
      </CardDescription>
      {action ? <CardAction>{action}</CardAction> : null}
    </CardHeader>
  );
}

/** A Guest's Members page: bex-api answers FORBIDDEN (ADR115 D4). */
export function OrganizationMembersForbidden() {
  const { t } = useTranslations();
  return (
    <PanelCenteredState
      icon={<Lock />}
      title={t("organizations.membersForbiddenTitle")}
      body={t("organizations.membersForbiddenBody")}
    />
  );
}

export function OrganizationMembersFailed() {
  const { t } = useTranslations();
  return (
    <PanelCenteredState
      icon={<AlertTriangle />}
      title={t("organizations.membersErrorTitle")}
      body={t("organizations.errorBody")}
    />
  );
}
