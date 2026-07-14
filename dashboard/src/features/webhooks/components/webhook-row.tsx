import { Loader2, Trash2 } from "lucide-react";
import { TableRow, TableCell } from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import { Switch } from "@/common/components/ui/switch";
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
import { WebhookDeliveriesDialog } from "@/features/webhooks/components/webhook-deliveries-dialog";
import type { WebhookEndpointView } from "@/features/webhooks/types";

export interface WebhookRowProps {
  entry: WebhookEndpointView;
  onToggle: (id: string, name: string, enabled: boolean) => Promise<boolean>;
  onDelete: (id: string, name: string) => Promise<boolean>;
  /** True while this row's toggle / delete is in flight. */
  toggling: boolean;
  deleting: boolean;
}

/**
 * One Webhooks row: name + URL, subscribed events, the enable/disable switch
 * (an auto-disabled endpoint shows its reason and re-arms from here), the
 * delivery-history dialog, and delete behind a confirmation.
 */
export function WebhookRow({
  entry,
  onToggle,
  onDelete,
  toggling,
  deleting,
}: WebhookRowProps) {
  const { t } = useTranslations();

  return (
    <TableRow>
      <TableCell className="max-w-[14rem]">
        <div className="truncate font-medium">{entry.name}</div>
        <div className="text-muted-foreground truncate font-mono text-xs">
          {entry.url}
        </div>
      </TableCell>
      <TableCell className="max-w-[14rem]">
        <div className="flex flex-wrap gap-1">
          {entry.eventTypes.map((eventType) => (
            <Badge key={eventType} variant="secondary" className="font-mono">
              {eventType}
            </Badge>
          ))}
        </div>
      </TableCell>
      <TableCell className="whitespace-nowrap">
        <div className="flex items-center gap-2">
          <Switch
            checked={entry.enabled}
            disabled={toggling}
            onCheckedChange={(v) => void onToggle(entry.id, entry.name, v)}
            aria-label={t("webhooks.toggle")}
          />
          {!entry.enabled && entry.disabledReason ? (
            <span
              className="text-muted-foreground max-w-[10rem] truncate text-xs"
              title={entry.disabledReason}
            >
              {entry.disabledReason}
            </span>
          ) : null}
        </div>
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        <WebhookDeliveriesDialog
          endpointId={entry.id}
          endpointName={entry.name}
        />
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              size="icon"
              variant="ghost"
              aria-label={t("webhooks.delete")}
              disabled={deleting}
            >
              {deleting ? (
                <Loader2 className="animate-spin" />
              ) : (
                <Trash2 className="text-destructive" />
              )}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("webhooks.deleteConfirmTitle", { name: entry.name })}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t("webhooks.deleteConfirmBody")}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("webhooks.deleteCancel")}</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => void onDelete(entry.id, entry.name)}
              >
                {t("webhooks.delete")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </TableCell>
    </TableRow>
  );
}
