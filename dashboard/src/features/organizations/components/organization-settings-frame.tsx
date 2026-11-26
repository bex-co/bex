import type { ComponentProps, ReactNode } from "react";
import {
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
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

/** The org API keys table's columns (Render's Name, Key, Created, Last used). */
const ORGANIZATION_KEY_COLUMNS = [
  "organizations.auditKeysColName",
  "organizations.auditKeysColKey",
  "organizations.auditKeysColCreated",
  "organizations.auditKeysColLastUsed",
] as const;

/**
 * Audit Logs → Audit Log API Keys (w1/m657), in the card and its skeleton
 * alike: Render's heading and description with the Create API key button
 * (`action`) beside them, then `children` (the keys, or why there are none).
 */
export function OrganizationAuditLogKeysSection({
  action,
  children,
}: {
  action: ReactNode;
  children: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <section
      className="space-y-4 border-t pt-6"
      data-testid="organization-audit-log-keys"
    >
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-1">
          <h3 className="text-sm font-medium">
            {t("organizations.auditKeysTitle")}
          </h3>
          <p className="text-muted-foreground text-sm">
            {t("organizations.auditKeysDescription")}
          </p>
        </div>
        <div className="shrink-0 self-start">{action}</div>
      </div>
      {children}
    </section>
  );
}

/**
 * The org API keys table with the rows passed as children; the skeleton
 * draws it with none. Name takes what the other columns leave, and below the
 * card's width the table scrolls in its card.
 */
export function OrganizationApiKeysTable({
  children,
  ...props
}: ComponentProps<"table">) {
  const { t } = useTranslations();
  return (
    <Table className="min-w-120" {...props}>
      <TableHeader>
        <TableRow>
          {ORGANIZATION_KEY_COLUMNS.map((label) => (
            <TableHead key={label}>{t(label)}</TableHead>
          ))}
          <TableHead className="w-12">
            <span className="sr-only">{t("common.colActions")}</span>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>{children}</TableBody>
    </Table>
  );
}

/** The line under the table when the org has no keys (Render's). */
export const ORGANIZATION_KEYS_EMPTY_CLASS =
  "bg-muted/40 text-muted-foreground rounded-lg p-4 text-center text-sm";

/** Audit Logs' export, for anyone but an Owner (Render's text). */
export function OrganizationAuditNoPermission() {
  const { t } = useTranslations();
  return (
    <p className="text-muted-foreground text-sm">
      {t("organizations.auditNoPermission")}
    </p>
  );
}

/** Audit Log API Keys, for anyone but an Owner (Render's text). */
export function OrganizationKeysNoPermission() {
  const { t } = useTranslations();
  return (
    <p className="text-muted-foreground text-sm">
      {t("organizations.auditKeysNoPermission")}
    </p>
  );
}
