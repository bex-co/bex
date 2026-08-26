import { Fragment, useState } from "react";
import {
  ChevronDown,
  ChevronUp,
  ExternalLink,
  Inbox,
  Loader2,
  RotateCw,
} from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
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
import { formatDateTime } from "@/common/lib/format";
import { config } from "@/config/config";
import { RelativeAge } from "@/common/components/relative-time";
import { useWebhookDeliveries } from "@/features/webhooks/hooks/use-webhook-deliveries";
import { useResendWebhookDelivery } from "@/features/webhooks/hooks/use-resend-webhook-delivery";
import { eventLabelKey } from "@/features/webhooks/event-catalog";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
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

export function WebhookDeliveriesCard({
  endpointId,
  endpointEnabled,
}: {
  endpointId: string;
  endpointEnabled: boolean;
}) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const { canManage } = useCapabilities();
  const canResend = endpointEnabled && canManage;
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
  const { resend, resendingAttemptId } = useResendWebhookDelivery();

  async function handleResend(attemptId: string) {
    const queued = await resend(endpointId, attemptId);
    if (queued) await refresh();
    return queued;
  }

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
        {error && deliveries.length === 0 ? (
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
                  <TableHead>
                    <span className="sr-only">{t("webhooks.colActions")}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {deliveries.map((d) => (
                  <DeliveryRow
                    key={d.id}
                    delivery={d}
                    ownerId={currentWorkspaceId}
                    canResend={canResend}
                    resending={resendingAttemptId === d.id}
                    onResend={handleResend}
                  />
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

function DeliveryRow({
  delivery,
  ownerId,
  canResend,
  resending,
  onResend,
}: {
  delivery: WebhookDeliveryView;
  ownerId: string | null;
  canResend: boolean;
  resending: boolean;
  onResend: (attemptId: string) => Promise<boolean>;
}) {
  const { t } = useTranslations();
  const [expanded, setExpanded] = useState(false);
  const [resendOpen, setResendOpen] = useState(false);
  const labelKey = eventLabelKey(delivery.eventType);
  const eventLabel = labelKey ? t(labelKey) : delivery.eventType;
  const evidence =
    delivery.requestBody || delivery.responseBody || delivery.transportError;
  const exactSentAt = formatDateTime(delivery.sentAt);
  let resultSummary = t("webhooks.noResponseEvidence");
  if (delivery.status === "pending") {
    resultSummary = t("webhooks.status.pending");
  } else if (delivery.statusCode > 0) {
    resultSummary = t("webhooks.httpStatus", { status: delivery.statusCode });
  } else if (delivery.transportError) {
    resultSummary = t("webhooks.transportError");
  }
  const eventURL =
    delivery.eventId && ownerId
      ? `${config.apiBaseUrl}/v1/events/${encodeURIComponent(delivery.eventId)}?ownerId=${encodeURIComponent(ownerId)}`
      : "";

  async function confirmResend() {
    const queued = await onResend(delivery.id);
    if (queued) setResendOpen(false);
  }

  return (
    <Fragment>
      <TableRow>
        <TableCell className="text-sm">
          {eventURL ? (
            <a
              href={eventURL}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 underline-offset-2 hover:underline"
              aria-label={t("webhooks.openSourceEvent", {
                id: delivery.eventId,
              })}
            >
              {eventLabel}
              <ExternalLink className="size-3" aria-hidden="true" />
            </a>
          ) : (
            eventLabel
          )}
        </TableCell>
        <TableCell className="max-w-[10rem] truncate font-mono text-sm">
          {delivery.serviceId || "—"}
        </TableCell>
        <TableCell>
          <Badge variant={statusVariant(delivery.status)}>
            {t(`webhooks.status.${delivery.status}`)}
          </Badge>
        </TableCell>
        <TableCell className="text-sm tabular-nums">
          {delivery.attemptNumber}
        </TableCell>
        <TableCell className="text-sm">
          {evidence ? (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2"
              onClick={() => setExpanded((value) => !value)}
              aria-expanded={expanded}
              aria-controls={`webhook-attempt-${delivery.id}`}
            >
              {resultSummary}
              {expanded ? <ChevronUp /> : <ChevronDown />}
            </Button>
          ) : (
            resultSummary
          )}
        </TableCell>
        <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
          {delivery.sentAt ? (
            <time dateTime={delivery.sentAt} className="flex flex-col">
              <RelativeAge value={delivery.sentAt} as="span" />
              <span className="text-xs">{exactSentAt ?? delivery.sentAt}</span>
            </time>
          ) : (
            "—"
          )}
        </TableCell>
        <TableCell className="text-right">
          {canResend && delivery.status === "failed" ? (
            <ConfirmDialog
              open={resendOpen}
              onOpenChange={setResendOpen}
              trigger={
                <Button variant="outline" size="sm" disabled={resending}>
                  {resending ? <Loader2 className="animate-spin" /> : null}
                  {t(resending ? "webhooks.resending" : "webhooks.resend")}
                </Button>
              }
              title={t("webhooks.resendConfirmTitle")}
              description={t("webhooks.resendConfirmBody")}
              cancelLabel={t("webhooks.resendCancel")}
              confirmLabel={
                <>
                  {resending ? <Loader2 className="animate-spin" /> : null}
                  {t(resending ? "webhooks.resending" : "webhooks.resend")}
                </>
              }
              // Re-sending a delivery is the primary action, not a destructive one.
              destructive={false}
              pending={resending}
              onConfirm={() => void confirmResend()}
            />
          ) : null}
        </TableCell>
      </TableRow>
      {expanded ? (
        <TableRow>
          <TableCell colSpan={7}>
            <div
              id={`webhook-attempt-${delivery.id}`}
              className="grid gap-4 md:grid-cols-2"
            >
              <EvidencePanel
                title={t("webhooks.requestPayload")}
                body={delivery.requestBody}
                empty={t("webhooks.noRequestEvidence")}
              />
              <EvidencePanel
                title={t("webhooks.endpointResponse")}
                body={delivery.responseBody}
                error={delivery.transportError}
                empty={t("webhooks.noResponseEvidence")}
              />
              <div className="text-muted-foreground space-y-1 text-xs md:col-span-2">
                <p>
                  {t("webhooks.attemptIdentity", {
                    attemptId: delivery.id,
                    eventId: delivery.eventId,
                  })}
                </p>
                <p>
                  {t("webhooks.parentDeliveryStatus", {
                    status: t(`webhooks.status.${delivery.parentStatus}`),
                  })}
                </p>
                {delivery.nextAttemptAt ? (
                  <p>
                    {t("webhooks.retryScheduled", {
                      date:
                        formatDateTime(delivery.nextAttemptAt) ??
                        delivery.nextAttemptAt,
                    })}
                  </p>
                ) : null}
              </div>
            </div>
          </TableCell>
        </TableRow>
      ) : null}
    </Fragment>
  );
}

function EvidencePanel({
  title,
  body,
  error,
  empty,
}: {
  title: string;
  body: string;
  error?: string;
  empty: string;
}) {
  return (
    <section className="space-y-2" aria-label={title}>
      <h3 className="text-sm font-medium">{title}</h3>
      {body ? (
        <pre className="bg-muted max-h-64 overflow-auto rounded-md p-3 text-xs whitespace-pre-wrap break-words">
          {body}
        </pre>
      ) : error ? null : (
        <p className="text-muted-foreground text-sm">{empty}</p>
      )}
      {error ? (
        <p className="text-destructive text-sm" role="alert">
          {error}
        </p>
      ) : null}
    </section>
  );
}
