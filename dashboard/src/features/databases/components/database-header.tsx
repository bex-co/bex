import { Link } from "@tanstack/react-router";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  DatastoreConnectMenu,
  DatastoreHeaderActions,
} from "@/common/components/datastore-connect-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { useConnectionRevealGate } from "@/features/capabilities/hooks/use-connection-reveal-gate";
import { fetchConnectionInfo } from "@/features/databases/hooks/use-connection-info";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import type { DatabaseDetailView } from "@/features/databases/types";

/** bex's Postgres docs, where Render's View docs opens its own. */
const DOCS_HREF = "https://bex.co/docs/platform/postgres";

/**
 * A database's plan in its header, opening the Plan page, as Render's plan
 * badge opens `/plan` (w3/m675). Its name comes from the plan catalogue.
 */
export function DatabasePlanBadge({
  database,
}: {
  database: Pick<DatabaseDetailView, "id" | "plan">;
}) {
  const { instanceTypes, error } = useDatabaseInstanceTypes();
  if (!database.plan) return null;
  const name = instanceTypes.find((it) => it.id === database.plan)?.name;
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
      to="/databases/$databaseId/plan"
      params={{ databaseId: database.id }}
      // A 46 px target around the 22 px badge, as the service header's.
      className="relative flex shrink-0 after:absolute after:-inset-x-1 after:-inset-y-3 after:content-['']"
    >
      <Badge variant="outline" className="hover:bg-accent">
        {name ?? database.plan}
      </Badge>
    </Link>
  );
}

/** A database header's View docs and Connect, Render's two actions. */
export function DatabaseHeaderActions({
  database,
}: {
  database: Pick<DatabaseDetailView, "id" | "public">;
}) {
  const { t } = useTranslations();
  const gate = useConnectionRevealGate("database", database.id);
  return (
    <DatastoreHeaderActions docsHref={DOCS_HREF}>
      <DatastoreConnectMenu
        id={database.id}
        // The Info card's own read, never another.
        readValues={fetchConnectionInfo}
        copy={{
          internalTitle: t("databases.connectInternalTitle"),
          externalTitle: t("databases.connectExternalTitle"),
          cliTitle: t("databases.connectCliTitle"),
          internalDocsHref: `${DOCS_HREF}#retrieve-connection-details`,
          externalDocsHref: `${DOCS_HREF}#restrict-access-and-manage-capacity`,
          copied: t("databases.copied"),
          copyError: t("databases.copyError"),
        }}
        externalOpen={database.public}
        cliCommand={`bex psql ${database.id}`}
        networkingLink={
          // The IP rules are on Info's Access control card.
          <Link
            to="/databases/$databaseId/info"
            params={{ databaseId: database.id }}
            hash="access-control"
            className="text-foreground underline underline-offset-2"
          >
            {t("databases.accessTitle")}
          </Link>
        }
        disabled={gate.disabled}
        disabledReason={gate.reason}
      />
    </DatastoreHeaderActions>
  );
}
