import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useIdleTimeout } from "@/features/services/hooks/use-idle-timeout";
import {
  autoSleepEligibleType,
  idleTimeoutOptions,
  planSleeps,
} from "@/features/services/lib/idle-timeout";

type TranslateFn = ReturnType<typeof useTranslations>["t"];

export interface IdleTimeoutRowProps {
  serviceId: string;
  serviceType: string;
  /** Render's plan spelling (e.g. "pro_plus"), or null for an untiered App. */
  plan: string | null;
  /** Current spec.idleTTLSeconds; 0 = platform default. */
  idleTTLSeconds: number;
}

/** Human label for a window in seconds (0 = platform default). */
function formatWindow(t: TranslateFn, seconds: number): string {
  if (seconds === 0) return t("services.idleTimeoutDefault");
  if (seconds % 3600 === 0)
    return t("services.idleTimeoutHours", { hours: seconds / 3600 });
  if (seconds % 60 === 0)
    return t("services.idleTimeoutMinutes", { minutes: seconds / 60 });
  return t("services.idleTimeoutSeconds", { seconds });
}

/**
 * The Settings "Idle timeout" control — a bex extension (Render's free spin-down
 * window is fixed, no user knob). Only public web services render the row: a
 * paid web plan shows an always-on notice, while a free/untiered web App gets a
 * preset select that persists to `spec.idleTTLSeconds` via `setIdleTimeout`.
 */
export function IdleTimeoutRow({
  serviceId,
  serviceType,
  plan,
  idleTTLSeconds,
}: IdleTimeoutRowProps) {
  const { t } = useTranslations();
  const { setIdleTimeout, busy } = useIdleTimeout();

  if (!autoSleepEligibleType(serviceType)) return null;

  const label = (
    <div className="text-sm text-muted-foreground">
      {t("services.settingsIdleTimeout")}
    </div>
  );

  // Paid web services never sleep (w1/m4) — no control, just the always-on notice.
  if (!planSleeps(plan)) {
    return (
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
        <div>
          {label}
          <div className="mt-1 text-sm text-muted-foreground italic">
            {t("services.settingsIdleTimeoutPaid")}
          </div>
        </div>
      </div>
    );
  }

  const options = idleTimeoutOptions(idleTTLSeconds);

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <div className="min-w-0">
        {label}
        <div className="mt-1 text-sm text-muted-foreground">
          {t("services.settingsIdleTimeoutHint")}
        </div>
      </div>
      <Select
        value={String(idleTTLSeconds)}
        disabled={busy}
        onValueChange={(v) => {
          const next = Number(v);
          if (next !== idleTTLSeconds) void setIdleTimeout(serviceId, next);
        }}
      >
        <SelectTrigger size="sm" className="w-full sm:w-40">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((s) => (
            <SelectItem key={s} value={String(s)}>
              {formatWindow(t, s)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
