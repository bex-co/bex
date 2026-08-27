import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useLocalDate } from "@/common/hooks/use-local-date";
import { WebhookEventChips } from "@/features/webhooks/components/webhook-event-chips";
import type { WebhookEndpointView } from "@/features/webhooks/types";

/**
 * The /webhook/$webhookId header, Render's shape
 * (docs/render-artifacts/webhooks-ui.md): kicker, name + enabled badge, id +
 * copy, URL + copy, subscribed-event chips with a show-more expander, and the
 * provenance line (createdBy comes from the API).
 */
export function WebhookDetailHeader({
  endpoint,
  creatorIdentity,
}: {
  endpoint: WebhookEndpointView;
  creatorIdentity?: string;
}) {
  const { t } = useTranslations();
  // Deferred to a post-hydration client render (the provenance line simply
  // appears once the viewer-local date is known) — the SSR container's UTC
  // clock must never be frozen on screen (w6/m107).
  const createdDate = useLocalDate(endpoint.createdAt);
  const creator = creatorIdentity ?? endpoint.createdBy;

  return (
    <div className="space-y-3 border-b px-4 py-4 sm:px-6">
      <div className="flex min-w-0 items-center gap-3">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/webhooks" aria-label={t("webhooks.backToList")}>
            <ArrowLeft />
          </Link>
        </Button>
        <div className="min-w-0">
          <p className="text-muted-foreground text-xs">
            {t("webhooks.detailKicker")}
          </p>
          <div className="flex items-center gap-2">
            <h1 className="truncate text-xl font-semibold">{endpoint.name}</h1>
            <Link
              to="/webhook/$webhookId/settings"
              params={{ webhookId: endpoint.id }}
              aria-label={t("webhooks.settingsStatus")}
            >
              <Badge
                variant={endpoint.enabled ? "success" : "secondary"}
                title={endpoint.enabled ? undefined : endpoint.disabledReason}
              >
                {endpoint.enabled
                  ? t("webhooks.enabledBadge")
                  : t("webhooks.disabledBadge")}
              </Badge>
            </Link>
          </div>
        </div>
      </div>
      <div className="text-muted-foreground flex flex-wrap items-center gap-x-6 gap-y-2 pl-12 text-sm">
        <span className="flex items-center gap-1">
          {t("webhooks.idLabel")}{" "}
          <code className="font-mono text-xs">{endpoint.id}</code>
          <CopyButton
            value={endpoint.id}
            label={t("webhooks.copyId")}
            successText={t("webhooks.copiedGeneric")}
            errorText={t("webhooks.copyError")}
          />
        </span>
        <span className="flex min-w-0 items-center gap-1">
          <a
            href={endpoint.url}
            target="_blank"
            rel="noreferrer"
            className="truncate font-mono text-xs underline-offset-2 hover:underline"
          >
            {endpoint.url}
          </a>
          <CopyButton
            value={endpoint.url}
            label={t("webhooks.copyUrl")}
            successText={t("webhooks.copiedGeneric")}
            errorText={t("webhooks.copyError")}
          />
        </span>
        {createdDate ? (
          <span>
            {creator
              ? t("webhooks.createdByOn", {
                  creator,
                  date: createdDate,
                })
              : t("webhooks.createdOn", { date: createdDate })}
          </span>
        ) : null}
      </div>
      <div className="pl-12">
        <WebhookEventChips
          eventTypes={endpoint.eventTypes}
          preview={5}
          className="items-center"
        />
      </div>
    </div>
  );
}
