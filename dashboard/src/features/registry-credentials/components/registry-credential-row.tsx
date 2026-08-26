import { TableRow, TableCell } from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { RevokeIconButton } from "@/common/components/revoke-icon-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { RelativeAge } from "@/common/components/relative-time";
import { formatDateLong } from "@/common/lib/format";
import { EditRegistryCredentialDialog } from "@/features/registry-credentials/components/edit-registry-credential-dialog";
import type { RegistryCredentialView } from "@/features/registry-credentials/types";

export interface RegistryCredentialRowProps {
  entry: RegistryCredentialView;
  onDelete: (id: string, name: string) => Promise<boolean>;
  /** True while this row's delete is in flight — disables its own control. */
  deleting: boolean;
}

/**
 * One registry-credentials row: name/host, username, an expiry-status badge
 * (w2/m14/t007 — omitted for a never-expiring "active" credential, since a
 * badge on every row would just be noise), and delete behind a confirmation.
 */
export function RegistryCredentialRow({
  entry,
  onDelete,
  deleting,
}: RegistryCredentialRowProps) {
  const { t } = useTranslations();

  return (
    <TableRow>
      <TableCell className="font-mono text-sm break-all">
        <div>{entry.name}</div>
        {entry.name !== entry.host && (
          <div className="text-muted-foreground text-xs">{entry.host}</div>
        )}
      </TableCell>
      <TableCell className="text-muted-foreground font-mono text-sm break-all">
        {entry.username}
      </TableCell>
      <TableCell className="text-sm whitespace-nowrap">
        <StatusBadge status={entry.status} expiresAt={entry.expiresAt} />
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        <RelativeAge value={entry.createdAt} />
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        <EditRegistryCredentialDialog entry={entry} />
        <RevokeIconButton
          label={t("registryCredentials.delete")}
          confirmTitle={t("registryCredentials.deleteConfirmTitle", {
            name: entry.name,
          })}
          confirmBody={t("registryCredentials.deleteConfirmBody")}
          cancelLabel={t("registryCredentials.deleteCancel")}
          confirmLabel={t("registryCredentials.delete")}
          pending={deleting}
          onConfirm={() => void onDelete(entry.id, entry.name)}
        />
      </TableCell>
    </TableRow>
  );
}

function StatusBadge({
  status,
  expiresAt,
}: {
  status: string;
  expiresAt: string | null;
}) {
  const { t } = useTranslations();
  if (status === "expired") {
    return (
      <Badge variant="destructive">{t("registryCredentials.expired")}</Badge>
    );
  }
  if (status === "expiring_soon") {
    return (
      <Badge variant="secondary">{t("registryCredentials.expiringSoon")}</Badge>
    );
  }
  if (expiresAt) {
    // A future date — formatRelativeAge assumes a past timestamp (it clamps
    // negative deltas to "now"), so render the plain date instead.
    return (
      <span className="text-muted-foreground">
        {t("registryCredentials.expiresOn", {
          date: formatDateLong(expiresAt) ?? expiresAt,
        })}
      </span>
    );
  }
  return (
    <span className="text-muted-foreground italic">
      {t("registryCredentials.neverExpires")}
    </span>
  );
}
