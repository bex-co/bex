import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useNotifyOnFail } from "@/features/services/hooks/use-notify-on-fail";

const NOTIFY_ON_FAIL_OPTIONS = [
  { value: "default", labelKey: "services.notifyOnFailOptionDefault" },
  { value: "notify", labelKey: "services.notifyOnFailOptionNotify" },
  { value: "ignore", labelKey: "services.notifyOnFailOptionIgnore" },
] as const;

export interface NotifyOnFailRowProps {
  serviceId: string;
  /** Current spec.notifyOnFail; null/empty means "default" (bex-api's own fallback). */
  notifyOnFail: string | null | undefined;
}

/**
 * Settings row for Render's per-service deploy-failure notification override
 * (w4/m21, docs/render-artifacts/notify-on-fail.md): "default" defers to each
 * workspace member's own Notifications preference (Settings → Notifications,
 * w3/m9), "ignore" mutes failure emails for this service for everyone,
 * "notify" forces a failure email to everyone regardless of their personal
 * preference. Governs deploy-failure notifications only — success emails
 * always follow each member's own preference.
 */
export function NotifyOnFailRow({
  serviceId,
  notifyOnFail,
}: NotifyOnFailRowProps) {
  const { t } = useTranslations();
  const { setNotifyOnFail, busy } = useNotifyOnFail();
  const current = notifyOnFail || "default";

  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <div className="text-sm text-muted-foreground">
          {t("services.notifyOnFailLabel")}
        </div>
        <div className="mt-1 text-sm text-muted-foreground">
          {t("services.notifyOnFailHint")}
        </div>
      </div>
      <Select
        value={current}
        disabled={busy}
        onValueChange={(next) => {
          if (next !== current) void setNotifyOnFail(serviceId, next);
        }}
      >
        <SelectTrigger size="sm" className="w-48">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {NOTIFY_ON_FAIL_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.labelKey)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
