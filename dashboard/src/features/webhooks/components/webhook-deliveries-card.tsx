import { useState } from "react";
import { Inbox, Loader2, RotateCw } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { Tabs, TabsList, TabsTrigger } from "@/common/components/ui/tabs";
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

type DeliveryFilter = "all" | "successful" | "failed";

function matchesFilter(
  delivery: WebhookDeliveryView,
  filter: DeliveryFilter,
): boolean {
  if (filter === "all") return true;
  return filter === "successful"
    ? delivery.status === "delivered"
    : delivery.status === "failed";
}

/**
 * The Activity tab's "Recent deliveries" (w1/m49/t004) — Render's shape:
 * All/Successful/Failed filter tabs, a manual refresh, newest first, keyset
 * "Load more". Replaces the w3/m11 delivery-history dialog; same hook, so
 * paging state resets when the page unmounts. Filtering is client-side over
 * the loaded rows — the deliveries query has no status param (a divergence
 * only if a filtered view must page server-side; revisit if that bites).
 */
export function WebhookDeliveriesCard({ endpointId }: { endpointId: string }) {
  const { t } = useTranslations();
  const [filter, setFilter] = useState<DeliveryFilter>("all");
  const { deliveries, loading, loadingMore, error, hasMore, loadMore, refresh } =
    useWebhookDeliveries(endpointId);

  const visible = deliveries.filter((d) => matchesFilter(d, filter));

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("webhooks.recentDeliveries")}</CardTitle>
        <CardDescription>{t("webhooks.recentDeliveriesHint")}</CardDescription>
        <CardAction>
          <Button
            variant="ghost"
            size="icon"
            aria-label={t("webhooks.refresh")}
            onClick={() => void refresh()}
          >
            <RotateCw />
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-4">
        <Tabs
          value={filter}
          onValueChange={(v) => setFilter(v as DeliveryFilter)}
        >
          <TabsList>
            <TabsTrigger value="all">{t("webhooks.filterAll")}</TabsTrigger>
            <TabsTrigger value="successful">
              {t("webhooks.filterSuccessful")}
            </TabsTrigger>
            <TabsTrigger value="failed">
              {t("webhooks.filterFailed")}
            </TabsTrigger>
          </TabsList>
        </Tabs>
        {error ? (
          <PanelCenteredState
            icon={<Inbox />}
            title={t("webhooks.historyErrorTitle")}
            body={t("webhooks.historyErrorBody")}
          />
        ) : loading ? (
          <PanelTableSkeleton />
        ) : visible.length === 0 ? (
          <PanelCenteredState
            icon={<Inbox />}
            title={t("webhooks.historyEmptyTitle")}
            body={t("webhooks.historyEmptyBody")}
          />
        ) : (
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
                {visible.map((d) => (
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
        )}
      </CardContent>
    </Card>
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
