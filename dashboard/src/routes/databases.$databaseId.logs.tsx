import { createFileRoute } from "@tanstack/react-router";
import { DatastoreLogsSkeleton } from "@/common/components/route-skeletons";
import { DatastoreLogViewer } from "@/features/logs/components/datastore-log-viewer";
import {
  datastoreLogSearchProps,
  parseDatastoreLogSearch,
  type DatastoreLogSearch,
} from "@/features/logs/lib/datastore-log-search";
import { databasePageHead } from "@/features/databases/lib/database-page-head";
import { useDatabaseDetail } from "@/features/databases/lib/database-detail";

/**
 * A Postgres database's Logs page, Render's `/d/<id>/logs` (w3/m641). Its
 * range persists in the shared `range`/`rangeStart`/`rangeEnd` URL keys
 * (w6/065, w4/703) and its text, instance and level filters beside them
 * (w4/228), so a filtered view survives a reload and can be shared. A Viewer
 * or a Billing member gets the role's reason (the viewer's LogsGate, w9/145).
 */
export const Route = createFileRoute("/databases/$databaseId/logs")({
  head: databasePageHead("databases.navLogs"),
  component: RouteComponent,
  pendingComponent: DatastoreLogsSkeleton,
  validateSearch: parseDatastoreLogSearch,
});

function RouteComponent() {
  const { database } = useDatabaseDetail();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  // Functional form: committed verbatim, so a patch's explicit undefineds
  // actually clear stale keys (w7/m42); the spread keeps the untouched keys.
  const write = (patch: DatastoreLogSearch) =>
    void navigate({
      search: (prev) => ({ ...prev, ...patch }),
      replace: true,
    });
  return (
    <DatastoreLogViewer
      kind="databases"
      resource={database.id}
      {...datastoreLogSearchProps(search, write)}
    />
  );
}
