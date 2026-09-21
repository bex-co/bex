import { useTranslations } from "@/common/hooks/use-translations";
import { usePort } from "@/features/services/hooks/use-port";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import {
  DEFAULT_SERVICE_PORT,
  MAX_SERVICE_PORT,
  MIN_SERVICE_PORT,
  parsePort,
} from "@/features/services/lib/port";

export interface ServicePortRowProps {
  serviceId: string;
  /** Current spec.port; null while loading, or for a type that binds none. */
  port: number | null | undefined;
}

/**
 * Settings row for the service port (w4/m121/t003) — the container port bex
 * routes to and injects as `$PORT`. Uses the shared edit-in-place row, same as
 * the health-check path beside it.
 *
 * Only rendered for web_service and private_service; the settings page gates
 * the section, and bex-api refuses `setPort` for every other type. Saving opens
 * a deploy (the backend bumps restartedAt), which the hint states up front.
 */
export function ServicePortRow({ serviceId, port }: ServicePortRowProps) {
  const { t } = useTranslations();
  const { setPort, busy } = usePort();
  const current = port != null ? String(port) : "";

  return (
    <EditableFieldRow
      label={t("services.settingsPort")}
      hint={t("services.settingsPortHint", {
        port: String(DEFAULT_SERVICE_PORT),
      })}
      value={current}
      placeholder={String(DEFAULT_SERVICE_PORT)}
      editLabel={t("services.settingsPortEdit")}
      type="number"
      min={MIN_SERVICE_PORT}
      max={MAX_SERVICE_PORT}
      busy={busy}
      validate={(draft) =>
        parsePort(draft) === null
          ? t("services.portRangeError", {
              min: String(MIN_SERVICE_PORT),
              max: String(MAX_SERVICE_PORT),
            })
          : null
      }
      onSave={async (value) => {
        const parsed = parsePort(value);
        // validate() already blocks Save on an unparseable draft; this is the
        // type narrowing, not a second gate.
        if (parsed === null) return false;
        return setPort(serviceId, parsed);
      }}
    />
  );
}
