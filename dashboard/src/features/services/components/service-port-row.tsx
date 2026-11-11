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
  /**
   * A service whose open ports come from its image (w6/m148, w4/m243): the
   * port set here picks the primary listener rather than the only one, so the
   * hint says the image's other declared TCP ports stay published, and a
   * blank port returns it to the image's own (w4/m323).
   */
  imagePorts?: boolean;
  /**
   * An image-port service has its port set (bex-api's portConfigured), so a
   * blank draft clears it. Without one the row is blank: the image chooses.
   */
  portConfigured?: boolean;
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
export function ServicePortRow({
  serviceId,
  port,
  imagePorts = false,
  portConfigured = false,
}: ServicePortRowProps) {
  const { t } = useTranslations();
  const { setPort, busy } = usePort();
  // An image-port service with no port set shows none: its image chooses the
  // port, and a resolved port shown here would read as a setting (w4/m323).
  const current =
    (imagePorts && !portConfigured) || port == null ? "" : String(port);

  return (
    <EditableFieldRow
      label={t("services.settingsPort")}
      hint={
        imagePorts
          ? t("services.settingsPortImageHint")
          : t("services.settingsPortHint", {
              port: String(DEFAULT_SERVICE_PORT),
            })
      }
      value={current}
      // An image-port service's primary comes from the image, so a blank
      // value must not suggest the configured-port default (w6/m148).
      placeholder={
        imagePorts
          ? t("services.createFieldPortFromImage")
          : String(DEFAULT_SERVICE_PORT)
      }
      editLabel={t("services.settingsPortEdit")}
      type="number"
      min={MIN_SERVICE_PORT}
      max={MAX_SERVICE_PORT}
      busy={busy}
      // Dirty-compare the parsed port, not the text: "03000" is port 3000, and
      // saving it would roll an identical deploy (w4/278). An image-port
      // service may clear its port to serve the image's port again, the fix
      // the rollout stall reason names (w4/m323).
      dirty={(draft) => {
        if (imagePorts && draft.trim() === "") return portConfigured;
        const parsed = parsePort(draft);
        return parsed !== null && parsed !== port;
      }}
      validate={(draft) =>
        (imagePorts && draft.trim() === "") || parsePort(draft) !== null
          ? null
          : t("services.portRangeError", {
              min: String(MIN_SERVICE_PORT),
              max: String(MAX_SERVICE_PORT),
            })
      }
      onSave={async (value) => {
        // bex-api reads port 0 as "use the image's port".
        if (imagePorts && value.trim() === "") return setPort(serviceId, 0);
        const parsed = parsePort(value);
        // validate() already blocks Save on an unparseable draft; this is the
        // type narrowing, not a second gate.
        if (parsed === null) return false;
        return setPort(serviceId, parsed);
      }}
    />
  );
}
