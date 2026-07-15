import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServiceNotifications } from "@/features/services/hooks/use-service-notifications";

const OPTIONS = [
  { value: "default", labelKey: "services.notificationsOptionDefault" },
  { value: "all", labelKey: "services.notificationsOptionAll" },
  { value: "failure", labelKey: "services.notificationsOptionFailure" },
  { value: "none", labelKey: "services.notificationsOptionNone" },
] as const;

export interface ServiceNotificationsRowProps {
  serviceId: string;
  notificationsToSend: string | null | undefined;
}

export function ServiceNotificationsRow({
  serviceId,
  notificationsToSend,
}: ServiceNotificationsRowProps) {
  const { t } = useTranslations();
  const { setNotificationsToSend, busy } = useServiceNotifications();
  const current = notificationsToSend || "default";
  return (
    <div className="flex flex-col items-start gap-4 sm:flex-row sm:justify-between">
      <div className="max-w-xl">
        <div className="text-sm font-medium">
          {t("services.notificationsLabel")}
        </div>
        <div className="mt-1 text-sm text-muted-foreground">
          {t("services.notificationsHint")}
        </div>
      </div>
      <Select
        value={current}
        disabled={busy}
        onValueChange={(next) => {
          if (next !== current) void setNotificationsToSend(serviceId, next);
        }}
      >
        <SelectTrigger size="sm" className="w-full sm:w-80">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.labelKey)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
