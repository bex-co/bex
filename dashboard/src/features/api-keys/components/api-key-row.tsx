import { TableRow, TableCell } from "@/common/components/ui/table";
import { RevokeIconButton } from "@/common/components/revoke-icon-button";
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspaceSubjectLabel } from "@/common/hooks/use-workspace-subject-label";
import { RelativeAge } from "@/common/components/relative-time";
import { EditApiKeyPermissionsDialog } from "@/features/api-keys/components/edit-api-key-permissions-dialog";
import { summarizePermissions } from "@/features/api-keys/lib/permissions";
import type {
  ApiKeyPermissionsView,
  ApiKeyView,
} from "@/features/api-keys/types";

export interface ApiKeyRowProps {
  entry: ApiKeyView;
  /**
   * The caller created this key: only then does the row carry the gear (a
   * grant is its creator's to change, w9/m539). An Admin's every-member view
   * passes false for anyone else's key, whose revoke confirmation then names
   * its creator.
   */
  own?: boolean;
  onRevoke: (id: string, name: string) => Promise<boolean>;
  /** True while this row's revoke is in flight — disables its own control. */
  revoking: boolean;
}

/**
 * One API Keys row: name, permissions with the gear that changes them
 * (w9/m496, Render's "Read-Only ⚙") on the caller's own key, client id,
 * provenance, usage ages, and revoke.
 */
export function ApiKeyRow({
  entry,
  own = true,
  onRevoke,
  revoking,
}: ApiKeyRowProps) {
  const { t } = useTranslations();
  const createdBy = useWorkspaceSubjectLabel(entry.createdBy ?? "");

  return (
    <TableRow>
      <TableCell className="font-mono text-sm">{entry.name}</TableCell>
      <TableCell className="text-sm whitespace-nowrap">
        <div className="flex items-center gap-1">
          <PermissionsSummary permissions={entry.permissions} />
          {own ? <EditApiKeyPermissionsDialog entry={entry} /> : null}
        </div>
      </TableCell>
      <TableCell className="font-mono text-sm">
        <div className="flex max-w-[14rem] items-center gap-1">
          <code className="min-w-0 flex-1 truncate" title={entry.id}>
            {entry.id}
          </code>
          <CopyButton
            value={entry.id}
            label={t("apiKeys.copyClientIdNamed", { name: entry.name })}
            successText={t("apiKeys.copied")}
            errorText={t("apiKeys.copyError")}
          />
        </div>
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        <RelativeAge value={entry.createdAt} />
      </TableCell>
      <TableCell className="text-muted-foreground max-w-[12rem] truncate text-sm">
        {entry.createdBy ? createdBy : "—"}
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        <RelativeAge
          value={entry.lastUsedAt}
          fallback={t("apiKeys.neverUsed")}
        />
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        <RevokeIconButton
          label={t("apiKeys.revokeNamed", { name: entry.name })}
          confirmTitle={t("apiKeys.revokeConfirmTitle", { name: entry.name })}
          confirmBody={
            !own && entry.createdBy
              ? t("apiKeys.revokeConfirmBodyOther", { creator: createdBy })
              : t("apiKeys.revokeConfirmBody")
          }
          cancelLabel={t("apiKeys.revokeCancel")}
          confirmLabel={t("apiKeys.revoke")}
          pending={revoking}
          // A key named before the length rule can still be very long: keep
          // Cancel and Revoke inside the viewport (w21/023).
          contentClassName="max-h-[85dvh] overflow-y-auto [overflow-wrap:anywhere]"
          onConfirm={() => void onRevoke(entry.id, entry.name)}
        />
      </TableCell>
    </TableRow>
  );
}

/** A key's grant in a word: All, Read only, No permissions, or Custom and how many. */
function PermissionsSummary({
  permissions,
}: {
  permissions: ApiKeyPermissionsView;
}) {
  const { t } = useTranslations();
  const summary = summarizePermissions(permissions);
  switch (summary.kind) {
    case "all":
      return <span>{t("apiKeys.permSummaryAll")}</span>;
    case "none":
      return (
        <span className="text-muted-foreground">
          {t("apiKeys.permSummaryNone")}
        </span>
      );
    case "readOnly":
    case "custom":
      return (
        <span className="flex items-baseline gap-1.5">
          <span>{t(COUNTED_SUMMARY_LABEL[summary.kind])}</span>
          <span className="text-muted-foreground text-xs">
            {t("apiKeys.permSummaryCount", { count: summary.count })}
          </span>
        </span>
      );
  }
}

const COUNTED_SUMMARY_LABEL = {
  readOnly: "apiKeys.permSummaryReadOnly",
  custom: "apiKeys.permSummaryCustom",
} as const;
