import type { ReactNode } from "react";
import {
  CardAction,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";

/** Every workspace row holds a small button's height, with or without one. */
export const WORKSPACE_ROW_CLASS = "h-12";

/** The Workspaces card's static header, with the Owner's create action. */
export function OrganizationWorkspacesCardHeader({
  action,
}: {
  action?: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("organizations.workspacesCardTitle")}</CardTitle>
      <CardDescription>
        {t("organizations.workspacesCardDescription")}
      </CardDescription>
      {action ? <CardAction>{action}</CardAction> : null}
    </CardHeader>
  );
}
