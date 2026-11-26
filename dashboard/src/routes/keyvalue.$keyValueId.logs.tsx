import { createFileRoute } from "@tanstack/react-router";
import { DatastoreLogsSkeleton } from "@/common/components/route-skeletons";
import { DatastoreLogViewer } from "@/features/logs/components/datastore-log-viewer";
import {
  datastoreLogSearchProps,
  parseDatastoreLogSearch,
  type DatastoreLogSearch,
} from "@/features/logs/lib/datastore-log-search";
import { keyValuePageHead } from "@/features/keyvalue/lib/key-value-page-head";
import { useKeyValueDetail } from "@/features/keyvalue/lib/key-value-detail";

/**
 * A Key Value instance's Logs page, Render's `/r/<id>/logs` (w3/m649). Its
 * range persists in the shared `range`/`rangeStart`/`rangeEnd` URL keys
 * (w6/065, w4/703) and its text, instance and level filters beside them
 * (w4/228), so a filtered view survives a reload and can be shared. A Viewer
 * or a Billing member gets the role's reason (the viewer's LogsGate, w9/145).
 */
export const Route = createFileRoute("/keyvalue/$keyValueId/logs")({
  head: keyValuePageHead("keyvalue.navLogs"),
  component: RouteComponent,
  pendingComponent: DatastoreLogsSkeleton,
  validateSearch: parseDatastoreLogSearch,
});

function RouteComponent() {
  const { keyValue } = useKeyValueDetail();
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
      kind="keyvalue"
      resource={keyValue.id}
      {...datastoreLogSearchProps(search, write)}
    />
  );
}
