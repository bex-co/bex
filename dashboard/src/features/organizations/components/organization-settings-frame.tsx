import type { ComponentProps, ReactNode } from "react";
import {
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { MAX_EXPORT_DAYS } from "@/features/audit/lib/audit-csv";
import { LoginRequirementRow } from "@/features/workspaces/components/login-requirement-row";
import { LOGIN_REQUIREMENT_SETTINGS } from "@/features/workspaces/lib/login-requirements";
import { ORGANIZATION_LOGIN_REQUIREMENT_SETTINGS } from "@/features/organizations/lib/organization-login-requirements";

/** The org name card's static header. */
export function OrganizationNameCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("organizations.nameTitle")}</CardTitle>
      <CardDescription>{t("organizations.nameDescription")}</CardDescription>
    </CardHeader>
  );
}

/** Org Settings → Audit Logs' static header (Render's audit-logs doc). */
export function OrganizationAuditLogsCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("organizations.auditTitle")}</CardTitle>
      <CardDescription>
        {t("organizations.auditDescription", { days: MAX_EXPORT_DAYS })}
      </CardDescription>
    </CardHeader>
  );
}

/** Org Settings -> Security's static header (w7/m466; Render's section of
 *  the same name, where it puts "Require Login Method"). */
export function OrganizationSecurityCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("organizations.securityTitle")}</CardTitle>
      <CardDescription>
        {t("organizations.securityDescription")}
      </CardDescription>
    </CardHeader>
  );
}

/**
 * One Security setting, in the card and in its skeleton alike: its row (the
 * label, then the switch, the state, or the switch's box: `row`'s `control`),
 * what it requires, then `children` (the refusal beside it).
 */
export function OrganizationLoginRequirementSection({
  requirement,
  children,
  ...row
}: {
  requirement: WorkspaceLoginRequirement;
  children?: ReactNode;
} & Pick<
  ComponentProps<typeof LoginRequirementRow>,
  "control" | "required" | "disabled" | "onCheckedChange"
>) {
  const { t } = useTranslations();
  const { section } = LOGIN_REQUIREMENT_SETTINGS[requirement];
  const copy = ORGANIZATION_LOGIN_REQUIREMENT_SETTINGS[requirement];
  return (
    <section className="space-y-2" data-login-requirement={section}>
      <LoginRequirementRow
        id={`organization-${section}`}
        region={`organization-${section}`}
        label={t(copy.labelKey)}
        {...row}
      />
      <p className="text-muted-foreground text-xs">{t(copy.descriptionKey)}</p>
      {children}
    </section>
  );
}

/** Why a Member or Guest reads the Security settings without a switch. */
export function OrganizationSecurityOwnerOnly() {
  const { t } = useTranslations();
  return (
    <p className="text-muted-foreground text-xs">
      {t("organizations.securityOwnerOnly")}
    </p>
  );
}

/** The name row: the field and, for an Owner, Save beside it. */
export const NAME_ROW_CLASS = "flex flex-col gap-3 sm:flex-row sm:items-end";
