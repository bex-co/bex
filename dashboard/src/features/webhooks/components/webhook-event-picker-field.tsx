import { AlertTriangle, Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { EventPicker } from "@/features/webhooks/components/event-picker";
import { useTranslations } from "@/common/hooks/use-translations";

export interface WebhookEventPickerFieldProps {
  eventTypes: string[];
  loading: boolean;
  error: Error | undefined;
  retry: () => Promise<unknown>;
  value: Set<string>;
  onChange: (next: Set<string>) => void;
  disabled?: boolean;
  describedBy?: string;
}

/** Shared actionable loading/error/empty shell around the served picker. */
export function WebhookEventPickerField({
  eventTypes,
  loading,
  error,
  retry,
  value,
  onChange,
  disabled,
  describedBy,
}: WebhookEventPickerFieldProps) {
  const { t } = useTranslations();

  if (loading && eventTypes.length === 0) {
    return (
      <p
        className="text-muted-foreground flex items-center gap-2 py-2 text-sm"
        role="status"
      >
        <Loader2 className="size-4 animate-spin" aria-hidden="true" />
        {t("webhooks.eventsLoading")}
      </p>
    );
  }
  if (error) {
    return (
      <div
        className="border-destructive/40 bg-destructive/5 space-y-2 rounded-md border p-3"
        role="alert"
      >
        <p className="flex items-center gap-2 text-sm">
          <AlertTriangle className="size-4" aria-hidden="true" />
          {t("webhooks.eventsLoadError")}
        </p>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => void retry()}
        >
          {t("webhooks.eventsRetry")}
        </Button>
      </div>
    );
  }
  if (eventTypes.length === 0) {
    return (
      <p
        className="text-muted-foreground rounded-md border p-3 text-sm"
        role="status"
      >
        {t("webhooks.eventsEmpty")}
      </p>
    );
  }
  return (
    <div aria-describedby={describedBy}>
      <EventPicker
        eventTypes={eventTypes}
        value={value}
        onChange={onChange}
        disabled={disabled}
      />
    </div>
  );
}
