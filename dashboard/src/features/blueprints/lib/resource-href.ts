import { serviceBaseForType } from "@/features/services/lib/service-base";

/**
 * The detail-page path for one of a blueprint's managed resources, or `null`
 * when this build has no page to send the reader to.
 *
 * A blueprint's Managed Resources table used to render the name as plain text
 * while the resource's own id sat unused beside it, so it was the one resource
 * table in the product that dead-ended: the reader had to leave the page and
 * search the resource by name (w4/101).
 *
 * `type` is the backend's closed set, produced in exactly four places
 * (`lego/backend/internal/apps/blueprint.go` — `effectiveType(App.Spec.Type)`
 * for services, plus the literals `postgres`, `key_value` and
 * `environment_group`). A service type maps through `serviceBaseForType`
 * rather than a second copy of the static-site rule, so a static site keeps
 * landing on its canonical `/static/<id>` base and cannot drift from the rest
 * of the product's links.
 *
 * An unrecognized type returns `null` — a future backend resource kind renders
 * as plain text exactly as everything did before, never as a broken href.
 */
export function blueprintResourceHref(
  type: string | null | undefined,
  id: string | null | undefined,
): string | null {
  if (!id) return null;
  switch (type) {
    case "postgres":
      return `/databases/${id}`;
    case "key_value":
      return `/keyvalue/${id}`;
    case "environment_group":
      return `/env-groups/${id}`;
    case "web_service":
    case "private_service":
    case "background_worker":
    case "cron_job":
    case "static_site":
      return `${serviceBaseForType(type)}/${id}`;
    default:
      return null;
  }
}
