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
  idleTimeoutOptions,
  planSleeps,
} from "@/features/services/lib/idle-timeout";

type TranslateFn = ReturnType<typeof useTranslations>["t"];

export interface IdleTimeoutRowProps {
  serviceId: string;
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
 * window is fixed, no user knob). Only free-tier Apps sleep, so a paid plan
 * shows an always-on notice instead of a control; a free/untiered App gets a
 * preset select that persists to `spec.idleTTLSeconds` via `setIdleTimeout`.
 */
export function IdleTimeoutRow({
  serviceId,
  plan,
  idleTTLSeconds,
}: IdleTimeoutRowProps) {
  const { t } = useTranslations();
  const { setIdleTimeout, busy } = useIdleTimeout();

  const label = (
    <div className="text-sm text-muted-foreground">
      {t("services.settingsIdleTimeout")}
    </div>
  );

  // Paid tiers never sleep (w1/m4) — no control, just the always-on notice.
  if (!planSleeps(plan)) {
    return (
      <div className="flex items-center justify-between gap-4">
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
    <div className="flex items-center justify-between gap-4">
      <div>
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
        <SelectTrigger size="sm" className="w-40">
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
