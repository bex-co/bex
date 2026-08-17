import { Fragment, useState } from "react";
import { ChevronDown, ChevronUp, Inbox, Loader2, RotateCw } from "lucide-react";
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
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  PanelCenteredState,
  PanelTableSkeleton,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRelativeAge } from "@/features/services/lib/format";
import { useWebhookDeliveries } from "@/features/webhooks/hooks/use-webhook-deliveries";
import { eventLabelKey } from "@/features/webhooks/event-catalog";
import type {
  WebhookDeliveryStatus,
  WebhookDeliveryView,
} from "@/features/webhooks/types";

type DeliveryFilter = "all" | "successful" | "failed";

function toAPITime(value: string): string | undefined {
  if (!value) return undefined;
  const parsed = new Date(value);
  return Number.isNaN(parsed.valueOf()) ? undefined : parsed.toISOString();
}

/**
 * The Activity tab's "Recent deliveries" (w1/m49/t004) — Render's shape:
 * All/Successful/Failed filter tabs, a manual refresh, newest first, keyset
 * "Load more". Replaces the w3/m11 delivery-history dialog; same hook, so
 * paging state resets when the page unmounts. Status and time bounds are sent
 * to the server, so a filtered view pages only matching rows.
 */
export function WebhookDeliveriesCard({ endpointId }: { endpointId: string }) {
  const { t } = useTranslations();
  const [filter, setFilter] = useState<DeliveryFilter>("all");
  const [sentAfter, setSentAfter] = useState("");
  const [sentBefore, setSentBefore] = useState("");
  const {
    deliveries,
    loading,
    loadingMore,
    error,
    hasMore,
    loadMore,
    refresh,
  } = useWebhookDeliveries(endpointId, {
    status:
      filter === "successful"
        ? "delivered"
        : filter === "failed"
          ? "failed"
          : undefined,
    sentAfter: toAPITime(sentAfter),
    sentBefore: toAPITime(sentBefore),
  });

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
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1">
            <Label htmlFor="webhook-sent-after">
              {t("webhooks.sentAfter")}
            </Label>
            <Input
              id="webhook-sent-after"
              type="datetime-local"
              value={sentAfter}
              onChange={(event) => setSentAfter(event.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="webhook-sent-before">
              {t("webhooks.sentBefore")}
            </Label>
            <Input
              id="webhook-sent-before"
              type="datetime-local"
              value={sentBefore}
              onChange={(event) => setSentBefore(event.target.value)}
            />
          </div>
        </div>
        {error ? (
          <PanelCenteredState
            icon={<Inbox />}
            title={t("webhooks.historyErrorTitle")}
            body={t("webhooks.historyErrorBody")}
          />
        ) : loading ? (
          <PanelTableSkeleton />
        ) : deliveries.length === 0 ? (
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
  const [expanded, setExpanded] = useState(false);
  const labelKey = eventLabelKey(delivery.eventType);
  const evidence = delivery.responseBody || delivery.lastError;
  return (
    <Fragment>
      <TableRow>
        <TableCell className="text-sm">
          {labelKey ? t(labelKey) : delivery.eventType}
        </TableCell>
        <TableCell className="max-w-[10rem] truncate font-mono text-sm">
          {delivery.serviceId || "—"}
        </TableCell>
        <TableCell>
          <Badge variant={statusVariant(delivery.status)}>
            {t(`webhooks.status.${delivery.status}`)}
          </Badge>
        </TableCell>
        <TableCell className="text-sm">{delivery.attemptCount}</TableCell>
        <TableCell className="text-sm">
          {evidence ? (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2"
              onClick={() => setExpanded((value) => !value)}
              aria-expanded={expanded}
            >
              {delivery.lastStatusCode > 0
                ? `HTTP ${delivery.lastStatusCode}`
                : t("webhooks.transportError")}
              {expanded ? <ChevronUp /> : <ChevronDown />}
            </Button>
          ) : delivery.lastStatusCode > 0 ? (
            `HTTP ${delivery.lastStatusCode}`
          ) : (
            "—"
          )}
        </TableCell>
        <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
          {formatRelativeAge(delivery.sentAt ?? delivery.createdAt)}
        </TableCell>
      </TableRow>
      {expanded ? (
        <TableRow>
          <TableCell colSpan={6}>
            {delivery.responseBody ? (
              <pre className="bg-muted max-h-48 overflow-auto rounded-md p-3 text-xs whitespace-pre-wrap break-words">
                {delivery.responseBody}
              </pre>
            ) : null}
            {delivery.lastError ? (
              <p className="text-destructive mt-2 text-sm">
                {delivery.lastError}
              </p>
            ) : null}
          </TableCell>
        </TableRow>
      ) : null}
    </Fragment>
  );
}
