import { Link } from "@tanstack/react-router";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  DatastoreConnectMenu,
  DatastoreHeaderActions,
} from "@/common/components/datastore-connect-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { useConnectionRevealGate } from "@/features/capabilities/hooks/use-connection-reveal-gate";
import { fetchConnectionInfo } from "@/features/keyvalue/hooks/use-connection-info";
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import type { KeyValueView } from "@/features/keyvalue/types";

/** bex's Key Value docs, where Render's View docs opens its own. */
const DOCS_HREF = "https://bex.co/docs/platform/key-value";

/**
 * An instance's plan in its header, opening Info's plan card, as Render's
 * plan badge opens `/info#compute` (w3/m675). Its name comes from the plan
 * catalogue.
 */
export function KeyValuePlanBadge({
  keyValue,
}: {
  keyValue: Pick<KeyValueView, "id" | "plan">;
}) {
  const { instanceTypes, error } = useKeyValueInstanceTypes();
  if (!keyValue.plan) return null;
  const name = instanceTypes.find((it) => it.id === keyValue.plan)?.name;
  // Until the catalogue answers, the badge holds its place: the server render
  // and the first paint have no catalogue yet, and the raw id would flash.
  // A failed catalogue, or a plan missing from it, shows the id.
  if (!name && instanceTypes.length === 0 && !error) {
    return (
      <Skeleton aria-hidden="true" className="h-[22px] w-14 rounded-full" />
    );
  }
  return (
    <Link
      to="/keyvalue/$keyValueId/info"
      params={{ keyValueId: keyValue.id }}
      hash="compute"
      // A 46 px target around the 22 px badge, as the service header's.
      className="relative flex shrink-0 after:absolute after:-inset-x-1 after:-inset-y-3 after:content-['']"
    >
      <Badge variant="outline" className="hover:bg-accent">
        {name ?? keyValue.plan}
      </Badge>
    </Link>
  );
}

/** An instance header's View docs and Connect, Render's two actions. */
export function KeyValueHeaderActions({
  keyValue,
}: {
  keyValue: Pick<KeyValueView, "id" | "public">;
}) {
  const { t } = useTranslations();
  const gate = useConnectionRevealGate("keyValue", keyValue.id);
  return (
    <DatastoreHeaderActions docsHref={DOCS_HREF}>
      <DatastoreConnectMenu
        id={keyValue.id}
        // The Info card's own read, never another.
        readValues={fetchConnectionInfo}
        copy={{
          internalTitle: t("keyvalue.connectInternalTitle"),
          externalTitle: t("keyvalue.connectExternalTitle"),
          cliTitle: t("keyvalue.connectCliTitle"),
          internalDocsHref: `${DOCS_HREF}#retrieve-the-correct-connection-string`,
          externalDocsHref: `${DOCS_HREF}#access-storage-and-lifecycle`,
          copied: t("keyvalue.copied"),
          copyError: t("keyvalue.copyError"),
        }}
        externalOpen={keyValue.public}
        cliCommand={`bex kv-cli ${keyValue.id}`}
        networkingLink={
          <Link
            to="/keyvalue/$keyValueId/info"
            params={{ keyValueId: keyValue.id }}
            hash="networking"
            className="text-foreground underline underline-offset-2"
          >
            {t("keyvalue.networkingTitle")}
          </Link>
        }
        disabled={gate.disabled}
        disabledReason={gate.reason}
      />
    </DatastoreHeaderActions>
  );
}
