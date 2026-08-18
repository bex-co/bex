import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";
import { TableRow, TableCell } from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { WebhookEventChips } from "@/features/webhooks/components/webhook-event-chips";
import type { WebhookEndpointView } from "@/features/webhooks/types";
import { formatRelativeAge } from "@/features/services/lib/format";
import { formatDateTime } from "@/common/lib/format";

export interface WebhookRowProps {
  entry: WebhookEndpointView;
  onToggle: (id: string, name: string, enabled: boolean) => Promise<boolean>;
  /** True while this row's toggle is in flight. */
  toggling: boolean;
  canManage: boolean;
}

/**
 * One Webhooks row: name + URL linking to the endpoint's page, subscribed
 * events, and the enable/disable switch (an auto-disabled endpoint shows its
 * reason and re-arms from here). Delivery history and delete moved to the
 * /webhook/$webhookId page (w1/m49/t004–t005) — Render's rows are links too.
 */
export function WebhookRow({
  entry,
  onToggle,
  toggling,
  canManage,
}: WebhookRowProps) {
  const { t } = useTranslations();
  const latestKey =
    entry.latestStatus === "delivered"
      ? "webhooks.latestDelivered"
      : entry.latestStatus === "failed" &&
          entry.latestParentStatus === "pending"
        ? "webhooks.latestRetrying"
        : entry.latestStatus === "failed"
          ? "webhooks.latestFailed"
          : "webhooks.latestNever";
  const latestVariant =
    entry.latestStatus === "delivered"
      ? "success"
      : entry.latestStatus === "failed" &&
          entry.latestParentStatus !== "pending"
        ? "destructive"
        : "secondary";
  const exactLatest = formatDateTime(entry.latestSentAt);

  return (
    <TableRow>
      <TableCell className="max-w-[14rem]">
        <Link
          to="/webhook/$webhookId"
          params={{ webhookId: entry.id }}
          className="block"
        >
          <div className="truncate font-medium hover:underline">
            {entry.name}
          </div>
          <div className="text-muted-foreground truncate font-mono text-xs">
            {entry.url}
          </div>
        </Link>
      </TableCell>
      <TableCell className="max-w-[14rem]">
        <WebhookEventChips eventTypes={entry.eventTypes} preview={3} />
      </TableCell>
      <TableCell className="whitespace-nowrap">
        <div className="flex flex-col items-start gap-1">
          <Badge variant={latestVariant}>{t(latestKey)}</Badge>
          {entry.latestSentAt ? (
            <time
              dateTime={entry.latestSentAt}
              title={exactLatest ?? entry.latestSentAt}
              className="text-muted-foreground flex flex-col text-xs"
            >
              <span>{formatRelativeAge(entry.latestSentAt)}</span>
              <span>{exactLatest ?? entry.latestSentAt}</span>
            </time>
          ) : null}
        </div>
      </TableCell>
      <TableCell className="whitespace-nowrap">
        <div className="flex items-center gap-2">
          {canManage ? (
            <Switch
              checked={entry.enabled}
              disabled={toggling}
              onCheckedChange={(v) => void onToggle(entry.id, entry.name, v)}
              aria-label={t("webhooks.toggle")}
            />
          ) : (
            <Badge variant={entry.enabled ? "success" : "secondary"}>
              {entry.enabled
                ? t("webhooks.enabledBadge")
                : t("webhooks.disabledBadge")}
            </Badge>
          )}
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
        <Button size="icon" variant="ghost" asChild>
          <Link
            to="/webhook/$webhookId"
            params={{ webhookId: entry.id }}
            aria-label={t("webhooks.view")}
          >
            <ChevronRight />
          </Link>
        </Button>
      </TableCell>
    </TableRow>
  );
}
