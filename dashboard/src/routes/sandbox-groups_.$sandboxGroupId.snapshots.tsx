import { createFileRoute } from "@tanstack/react-router";
import { SnapshotsPageSkeleton } from "@/features/sandbox-group-pages/components/group-page-skeletons";
import { SnapshotsPage } from "@/features/sandbox-group-pages/components/snapshots-page";
import { sandboxGroupPageHead } from "@/features/sandbox-group-pages/lib/page-head";

export interface SnapshotsSearch {
  /** The snapshot whose drawer is open. */
  snapshot?: string;
}

const SNAPSHOT_ID = /^snp-[0-9a-v]{20}$/;

/**
 * A sandbox group's Snapshots page, Render's `snapshots/*` (w1/m658);
 * `?snapshot=<snp-…>` opens a snapshot's drawer.
 */
export const Route = createFileRoute("/sandbox-groups_/$sandboxGroupId/snapshots")({
  head: sandboxGroupPageHead("sandboxGroupPages.navSnapshots"),
  component: RouteComponent,
  pendingComponent: SnapshotsPageSkeleton,
  validateSearch: (search: Record<string, unknown>): SnapshotsSearch => ({
    snapshot:
      typeof search.snapshot === "string" && SNAPSHOT_ID.test(search.snapshot)
        ? search.snapshot
        : undefined,
  }),
});

function RouteComponent() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <SnapshotsPage
      openSnapshotId={search.snapshot}
      onOpen={(snapshot) => void navigate({ search: { snapshot } })}
      onClose={() => void navigate({ search: { snapshot: undefined } })}
    />
  );
}
