import { useIntl } from "react-intl";
import type { UiText } from "@ory/client-fetch";
import { uiTextToFormattedMessage } from "@ory/elements-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { conjunctionList } from "@/common/lib/format/language";

/**
 * The refusals of bex-api's Kratos settings guard (w7/m465,
 * `lego/backend/internal/settingsguard`), by the message id it sends: Render's
 * "can't change their email address" and "can't disconnect" Google while the
 * member belongs to a workspace that requires Google login. Kratos leaves the
 * 41xxxxx range to web hooks, so Ory Elements has no translation for them and
 * would show the guard's English `text`; the dashboard words them itself.
 * `context.workspaces` is the list of workspaces to leave first.
 */
const SETTINGS_GUARD_MESSAGES: ReadonlyMap<number, string> = new Map([
  // Attached to the `traits.email` node of the settings flow's profile form.
  [4100001, "auth.settingsGuardEmailChangeRefused"],
  // A flow-level message: the settings flow shows it as a toast.
  [4100002, "auth.settingsGuardUnlinkRefused"],
]);

/** The workspace names a guard refusal carries, or null when it carries none. */
function guardWorkspaces(message: UiText): string[] | null {
  const workspaces = (message.context as { workspaces?: unknown } | undefined)
    ?.workspaces;
  if (!Array.isArray(workspaces)) return null;
  const names = workspaces.filter(
    (name): name is string => typeof name === "string" && name !== "",
  );
  return names.length > 0 ? names : null;
}

/**
 * A Kratos UI message in the dashboard's language: the settings guard's
 * refusals from the dashboard's own catalog, with the workspaces listed the
 * language's way ("A, B and C"); everything else as Ory Elements words it. A
 * guard refusal that names no workspace keeps the server's text, which does.
 * Must render inside an Ory flow (it reads Ory's react-intl context).
 */
export function useOryMessageText(message: UiText): string {
  const intl = useIntl();
  const { t, i18n } = useTranslations();
  const key = SETTINGS_GUARD_MESSAGES.get(message.id);
  const names = key ? guardWorkspaces(message) : null;
  if (!key || !names) return uiTextToFormattedMessage(message, intl);
  return t(key, { workspaces: conjunctionList(i18n.language).format(names) });
}
