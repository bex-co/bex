import { TableRow, TableCell } from "@/common/components/ui/table";
import { RevokeIconButton } from "@/common/components/revoke-icon-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { RelativeAge } from "@/common/components/relative-time";
import type { ApiKeyView } from "@/features/api-keys/types";

export interface ApiKeyRowProps {
  entry: ApiKeyView;
  onRevoke: (id: string, name: string) => Promise<boolean>;
  /** True while this row's revoke is in flight — disables its own control. */
  revoking: boolean;
}

/** One API Keys row: name, provenance, usage ages, and revoke behind a confirm. */
export function ApiKeyRow({ entry, onRevoke, revoking }: ApiKeyRowProps) {
  const { t } = useTranslations();

  return (
    <TableRow>
      <TableCell className="font-mono text-sm break-all">
        {entry.name}
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        <RelativeAge value={entry.createdAt} />
      </TableCell>
      <TableCell className="text-muted-foreground max-w-[12rem] truncate font-mono text-sm">
        {entry.createdBy ?? "—"}
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        <RelativeAge
          value={entry.lastUsedAt}
          fallback={t("apiKeys.neverUsed")}
        />
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        <RevokeIconButton
          label={t("apiKeys.revoke")}
          confirmTitle={t("apiKeys.revokeConfirmTitle", { name: entry.name })}
          confirmBody={t("apiKeys.revokeConfirmBody")}
          cancelLabel={t("apiKeys.revokeCancel")}
          confirmLabel={t("apiKeys.revoke")}
          pending={revoking}
          onConfirm={() => void onRevoke(entry.id, entry.name)}
        />
      </TableCell>
    </TableRow>
  );
}
