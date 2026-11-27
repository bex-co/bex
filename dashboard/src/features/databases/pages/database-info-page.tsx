import { lazy, Suspense } from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { CardSkeleton } from "@/common/components/detail-skeletons";
import { DeferredMount } from "@/common/components/deferred-mount";
import { DatabaseMetadataCard } from "@/features/databases/components/database-metadata-card";
import { ConnectionInfoPanel } from "@/features/databases/components/connection-info-panel";
import { DatabaseDangerActions } from "@/features/databases/components/database-danger-actions";
import { useDatabaseDetail } from "@/features/databases/lib/database-detail";
import { useMovedAnchorRedirect } from "@/features/databases/lib/database-pages";
import { deriveStatus, sqlConsoleBlock } from "@/features/databases/lib/status";

const AccessControlPanel = lazy(() =>
  import("@/features/databases/components/access-control-panel").then((m) => ({
    default: m.AccessControlPanel,
  })),
);
const SQLConsole = lazy(() =>
  import("@/features/databases/components/sql-console").then((m) => ({
    default: m.SQLConsole,
  })),
);

/**
 * A Postgres database's Info page, at its bare URL and at `/info`, as on
 * Render (w3/m641, docs/render-artifacts/postgres-dashboard.md). Render's Info
 * holds General, Connections, Credentials, Networking and the delete card
 * (with Restart and Suspend), in that order; bex's Details, Connections,
 * Access control (credentials and the IP allow list) and danger zone follow
 * it, and its SQL console, which Render has no counterpart for, sits before
 * the danger zone. Each card keeps the id the old Overview gave it, so its
 * anchor still lands on it.
 */
export function DatabaseInfoPage() {
  const { database, refetch, lifecycle } = useDatabaseDetail();
  const navigate = useNavigate();
  const router = useRouter();
  // A full load of an old Overview anchor whose card moved off Info
  // (`#recovery`, `#plan`, …) goes on to that card's page.
  useMovedAnchorRedirect(database.id);

  return (
    <>
      <section id="metadata" className="scroll-mt-6">
        <DatabaseMetadataCard
          database={database}
          onRenamed={() => void router.invalidate()}
        />
      </section>
      <section id="connection" className="scroll-mt-6">
        {/* A revealed snapshot belongs to one publication state: when a
            confirmed access change flips `public`, remount so the old external
            URL is no longer copyable and a fresh Reveal is required (w4/222).
            Renames and status polls keep it. */}
        <ConnectionInfoPanel
          key={`${database.id}:${database.public}`}
          id={database.id}
        />
      </section>
      <section id="access-control" className="scroll-mt-6">
        <DeferredMount hashId="access-control" minHeight={240}>
          <Suspense fallback={<CardSkeleton rows={3} />}>
            <AccessControlPanel
              database={database}
              onChanged={() => void refetch()}
            />
          </Suspense>
        </DeferredMount>
      </section>
      <section id="sql-console" className="scroll-mt-6">
        {/* The mounted console's height (DatabaseInfoSkeleton's): 439 px
            stacked at a 390 px phone, 375 px from sm. */}
        <DeferredMount
          hashId="sql-console"
          minHeight="var(--sql-reserve)"
          className="[--sql-reserve:439px] sm:[--sql-reserve:375px]"
        >
          <Suspense fallback={<CardSkeleton rows={4} />}>
            <SQLConsole
              key={`sql-${database.id}`}
              id={database.id}
              blocked={sqlConsoleBlock(deriveStatus(database).key)}
            />
          </Suspense>
        </DeferredMount>
      </section>
      <section id="danger-zone" className="scroll-mt-6">
        <DatabaseDangerActions
          database={database}
          onDeleted={() => void navigate({ to: "/", replace: true })}
          lifecycle={lifecycle}
        />
      </section>
    </>
  );
}
