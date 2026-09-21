import { Link } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";

const LINK_CLASS =
  "font-medium underline underline-offset-2 hover:no-underline" as const;

export interface ReservedEnvKeyNoticeProps {
  /** The reserved key as typed (today only `PORT`). */
  envKey: string;
  /** Which feature's copy of the sentence to render; both say the same thing. */
  messageKey?: "services.envReservedKey" | "envGroups.reservedKey";
  /**
   * The service whose Settings → Port control the sentence should open. Absent
   * on the env-group editor, where the group can be linked to many services
   * (or to none yet), so there is no single port to send the reader at — that
   * case links to the services list instead and lets them pick.
   */
  serviceId?: string;
  /**
   * Id of a port field on the SAME page — the create wizard's own `svc-port`.
   * There is no service to navigate to yet there, so the sentence scrolls to
   * the field the reader is already looking at.
   */
  portFieldId?: string;
}

/**
 * The inline refusal shown when someone types a key bex owns (w2/m95 t003),
 * with the control its own instruction names (w4/m121/t003).
 *
 * bex-api's `core.ReservedEnvKeySentence` tells every caller to "change the
 * service's port field instead". Until w4/m121 that field existed on no
 * dashboard surface, so the sentence was a dead end; it now resolves to the
 * port control, and this component is the link. The wording tracks the server's
 * sentence — the text surfaces name the REST/GraphQL/MCP verbs because they
 * cannot link; here the link IS the "or Settings in the dashboard" clause.
 */
export function ReservedEnvKeyNotice({
  envKey,
  messageKey = "services.envReservedKey",
  serviceId,
  portFieldId,
}: ReservedEnvKeyNoticeProps) {
  const { t } = useTranslations();

  return (
    <p className="text-destructive text-xs" role="alert">
      {t(messageKey, { key: envKey })}{" "}
      {portFieldId ? (
        <a href={`#${portFieldId}`} className={LINK_CLASS}>
          {t("services.envReservedKeyFieldLink")}
        </a>
      ) : serviceId ? (
        <Link
          to="/services/$serviceId/settings"
          params={{ serviceId }}
          hash="port"
          className={LINK_CLASS}
        >
          {t("services.envReservedKeySettingsLink")}
        </Link>
      ) : (
        <Link to="/" className={LINK_CLASS}>
          {t("services.envReservedKeyServicesLink")}
        </Link>
      )}
    </p>
  );
}
