import { Loader2, Trash2 } from "lucide-react";
import { TableRow, TableCell } from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/common/components/ui/alert-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRelativeAge } from "@/features/services/lib/format";
import type { ApiKeyView } from "@/features/api-keys/types";

export interface ApiKeyRowProps {
  entry: ApiKeyView;
  onRevoke: (id: string, name: string) => Promise<boolean>;
  /** True while this row's revoke is in flight — disables its own control. */
  revoking: boolean;
}

/** One API Keys row: name, created age, and revoke behind a confirmation. */
export function ApiKeyRow({ entry, onRevoke, revoking }: ApiKeyRowProps) {
  const { t } = useTranslations();

  return (
    <TableRow>
      <TableCell className="font-mono text-sm break-all">{entry.name}</TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        {formatRelativeAge(entry.createdAt)}
      </TableCell>
      <TableCell className="text-muted-foreground max-w-[12rem] truncate font-mono text-sm">
        {entry.createdBy ?? "—"}
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        {entry.lastUsedAt ? formatRelativeAge(entry.lastUsedAt) : t("apiKeys.neverUsed")}
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              size="icon"
              variant="ghost"
              aria-label={t("apiKeys.revoke")}
              disabled={revoking}
            >
              {revoking ? (
                <Loader2 className="animate-spin" />
              ) : (
                <Trash2 className="text-destructive" />
              )}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("apiKeys.revokeConfirmTitle", { name: entry.name })}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t("apiKeys.revokeConfirmBody")}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("apiKeys.revokeCancel")}</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => void onRevoke(entry.id, entry.name)}
              >
                {t("apiKeys.revoke")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </TableCell>
    </TableRow>
  );
}
