import { History, Inbox, Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import {
  PanelCenteredState,
  PanelTableSkeleton,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRelativeAge } from "@/features/services/lib/format";
import { useWebhookDeliveries } from "@/features/webhooks/hooks/use-webhook-deliveries";
import type {
  WebhookDeliveryStatus,
  WebhookDeliveryView,
} from "@/features/webhooks/types";

export interface WebhookDeliveriesDialogProps {
  endpointId: string;
  endpointName: string;
}

/**
 * Per-endpoint delivery history (w3/m11/t007): status/attempts/response-code
 * table, newest first, "Load more" keyset paging (the audit-log pattern).
 * The hook mounts with the dialog content, so paging state resets on close.
 */
export function WebhookDeliveriesDialog({
  endpointId,
  endpointName,
}: WebhookDeliveriesDialogProps) {
  const { t } = useTranslations();
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button size="icon" variant="ghost" aria-label={t("webhooks.history")}>
          <History />
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {t("webhooks.historyTitle", { name: endpointName })}
          </DialogTitle>
          <DialogDescription>{t("webhooks.historyBody")}</DialogDescription>
        </DialogHeader>
        <DeliveriesTable endpointId={endpointId} />
      </DialogContent>
    </Dialog>
  );
}

function DeliveriesTable({ endpointId }: { endpointId: string }) {
  const { t } = useTranslations();
  const { deliveries, loading, loadingMore, error, hasMore, loadMore } =
    useWebhookDeliveries(endpointId);

  if (error) {
    return (
      <PanelCenteredState
        icon={<Inbox />}
        title={t("webhooks.historyErrorTitle")}
        body={t("webhooks.historyErrorBody")}
      />
    );
  }
  if (loading) {
    return <PanelTableSkeleton />;
  }
  if (deliveries.length === 0) {
    return (
      <PanelCenteredState
        icon={<Inbox />}
        title={t("webhooks.historyEmptyTitle")}
        body={t("webhooks.historyEmptyBody")}
      />
    );
  }
  return (
    <div className="space-y-3">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("webhooks.colEvent")}</TableHead>
            <TableHead>{t("webhooks.colService")}</TableHead>
            <TableHead>{t("webhooks.colStatus")}</TableHead>
            <TableHead>{t("webhooks.colAttempts")}</TableHead>
            <TableHead>{t("webhooks.colResponse")}</TableHead>
            <TableHead>{t("webhooks.colWhen")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {deliveries.map((d) => (
            <DeliveryRow key={d.id} delivery={d} />
          ))}
        </TableBody>
      </Table>
      {hasMore ? (
        <Button
          variant="outline"
          size="sm"
          onClick={() => void loadMore()}
          disabled={loadingMore}
        >
          {loadingMore ? <Loader2 className="animate-spin" /> : null}
          {t("webhooks.loadMore")}
        </Button>
      ) : null}
    </div>
  );
}

function statusVariant(
  status: WebhookDeliveryStatus,
): "success" | "destructive" | "secondary" {
  switch (status) {
    case "delivered":
      return "success";
    case "failed":
      return "destructive";
    default:
      return "secondary";
  }
}

function DeliveryRow({ delivery }: { delivery: WebhookDeliveryView }) {
  const { t } = useTranslations();
  return (
    <TableRow>
      <TableCell className="font-mono text-sm">{delivery.eventType}</TableCell>
      <TableCell className="max-w-[10rem] truncate font-mono text-sm">
        {delivery.serviceId || "—"}
      </TableCell>
      <TableCell>
        <Badge variant={statusVariant(delivery.status)}>
          {t(`webhooks.status.${delivery.status}`)}
        </Badge>
      </TableCell>
      <TableCell className="text-sm">{delivery.attemptCount}</TableCell>
      <TableCell
        className="max-w-[12rem] truncate text-sm"
        title={delivery.lastError || undefined}
      >
        {delivery.lastStatusCode > 0 ? delivery.lastStatusCode : "—"}
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        {formatRelativeAge(delivery.createdAt)}
      </TableCell>
    </TableRow>
  );
}
