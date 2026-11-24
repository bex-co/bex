import { Link } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useMaxShutdownDelay } from "@/features/services/hooks/use-max-shutdown-delay";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";

export interface MaxShutdownDelayRowProps {
  serviceId: string;
  /** Effective value from bex-api; null on legacy responses defaults to 30. */
  maxShutdownDelaySeconds: number | null | undefined;
  onChanged?: () => void;
  /** A release has gone live; false means nothing keeps serving (w4/548). */
  live?: boolean;
  /**
   * The service has a persistent disk, so it keeps the 30-second default and
   * the row is read-only, as on Render (w6/m548).
   */
  hasDisk?: boolean;
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

/**
 * Render-style edit-in-place row for the graceful-shutdown window (1–300s).
 * On a service with a disk it reads 30 and cannot be edited: Render says why
 * in the hint's place ("You can't change the shutdown delay for a service with
 * a persistent disk"), so the row keeps its shape and the settings skeleton's
 * height still holds.
 */
export function MaxShutdownDelayRow({
  serviceId,
  maxShutdownDelaySeconds,
  onChanged,
  live = true,
  hasDisk = false,
}: MaxShutdownDelayRowProps) {
  const { t } = useTranslations();
  const { setMaxShutdownDelay, busy } = useMaxShutdownDelay();
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");
  const current = hasDisk ? 30 : (maxShutdownDelaySeconds ?? 30);
  const hint = hasDisk ? (
    <>
      {t("services.maxShutdownDelayDiskReason")}{" "}
      <Link
        to="/services/$serviceId/disk"
        params={{ serviceId }}
        className="text-primary font-medium underline-offset-4 hover:underline"
      >
        {t("services.maxShutdownDelayDiskLink")}
      </Link>
    </>
  ) : (
    t("services.settingsMaxShutdownDelayHint")
  );

  return (
    <EditableFieldRow
      label={t("services.settingsMaxShutdownDelay")}
      hint={hint}
      disabled={hasDisk || createDenied}
      // A disk's own reason is the hint; the role's is a line beneath.
      disabledReason={createReason}
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
      // The save starts a deploy, so it says so first (w4/616).
      confirm={{
        title: (value) => t("services.maxShutdownDelayConfirmTitle", { value }),
        body: t(
          live
            ? "services.settingRedeployConfirmBody"
            : "services.settingRedeployConfirmBodyNeverLive",
        ),
      }}
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
