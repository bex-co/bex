import { useTranslations } from "@/common/hooks/use-translations";
import { useMaxShutdownDelay } from "@/features/services/hooks/use-max-shutdown-delay";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";

export interface MaxShutdownDelayRowProps {
  serviceId: string;
  /** Effective value from bex-api; null on legacy responses defaults to 30. */
  maxShutdownDelaySeconds: number | null | undefined;
  onChanged?: () => void;
}

/** Render-style edit-in-place row for the graceful-shutdown window (1–300s). */
export function MaxShutdownDelayRow({
  serviceId,
  maxShutdownDelaySeconds,
  onChanged,
}: MaxShutdownDelayRowProps) {
  const { t } = useTranslations();
  const { setMaxShutdownDelay, busy } = useMaxShutdownDelay();
  const current = maxShutdownDelaySeconds ?? 30;

  return (
    <EditableFieldRow
      label={t("services.settingsMaxShutdownDelay")}
      hint={t("services.settingsMaxShutdownDelayHint")}
      value={String(current)}
      editLabel={t("services.maxShutdownDelayEdit")}
      type="number"
      min={1}
      max={300}
      step={1}
      mono
      busy={busy}
      // Only an in-range integer other than the current value is savable.
      dirty={(value) => {
        const parsed = Number(value);
        return (
          Number.isInteger(parsed) &&
          parsed >= 1 &&
          parsed <= 300 &&
          parsed !== current
        );
      }}
      onSave={async (value) => {
        const ok = await setMaxShutdownDelay(serviceId, Number(value));
        if (ok) onChanged?.();
        return ok;
      }}
    />
  );
}
