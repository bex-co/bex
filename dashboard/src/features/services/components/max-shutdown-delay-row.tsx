import { useTranslations } from "@/common/hooks/use-translations";
import { useMaxShutdownDelay } from "@/features/services/hooks/use-max-shutdown-delay";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";

export interface MaxShutdownDelayRowProps {
  serviceId: string;
  /** Effective value from bex-api; null on legacy responses defaults to 30. */
  maxShutdownDelaySeconds: number | null | undefined;
  onChanged?: () => void;
}

const MIN_DELAY = 1;
const MAX_DELAY = 300;

/** The draft as seconds, or null unless it is a whole number from 1 to 300. */
function parseDelay(draft: string): number | null {
  if (draft.trim() === "") return null;
  const parsed = Number(draft);
  return Number.isInteger(parsed) && parsed >= MIN_DELAY && parsed <= MAX_DELAY
    ? parsed
    : null;
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
      min={MIN_DELAY}
      max={MAX_DELAY}
      step={1}
      mono
      busy={busy}
      // Dirty means "differs from the current value"; an out-of-range draft is
      // reported by validate() instead of silently greying out Save (w4/494).
      dirty={(value) => {
        const parsed = parseDelay(value);
        return parsed !== null && parsed !== current;
      }}
      validate={(value) =>
        parseDelay(value) === null
          ? t("services.maxShutdownDelayRangeError", {
              min: String(MIN_DELAY),
              max: String(MAX_DELAY),
            })
          : null
      }
      onSave={async (value) => {
        const parsed = parseDelay(value);
        // validate() already blocks Save on an invalid draft; this is the type
        // narrowing, not a second gate.
        if (parsed === null) return false;
        const ok = await setMaxShutdownDelay(serviceId, parsed);
        if (ok) onChanged?.();
        return ok;
      }}
    />
  );
}
