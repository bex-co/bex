import { useState } from "react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { eventLabelKey } from "@/features/webhooks/event-catalog";

export function WebhookEventChips({
  eventTypes,
  preview,
  className,
}: {
  eventTypes: string[];
  preview: number;
  className?: string;
}) {
  const { t } = useTranslations();
  const [expanded, setExpanded] = useState(false);
  if (eventTypes.length === 0) {
    return <Badge variant="secondary">{t("webhooks.allEvents")}</Badge>;
  }

  const shown = expanded ? eventTypes : eventTypes.slice(0, preview);
  const hidden = eventTypes.length - shown.length;
  return (
    <div className={cn("flex flex-wrap gap-1", className)}>
      {shown.map((eventType) => {
        const labelKey = eventLabelKey(eventType);
        return (
          <Badge key={eventType} variant="secondary">
            {labelKey ? t(labelKey) : eventType}
          </Badge>
        );
      })}
      {hidden > 0 ? (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-6 px-2 text-xs"
          onClick={() => setExpanded(true)}
          aria-expanded={false}
        >
          {t("webhooks.showMore", { count: hidden })}
        </Button>
      ) : null}
      {expanded && eventTypes.length > preview ? (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-6 px-2 text-xs"
          onClick={() => setExpanded(false)}
          aria-expanded={true}
        >
          {t("webhooks.showLess")}
        </Button>
      ) : null}
    </div>
  );
}
