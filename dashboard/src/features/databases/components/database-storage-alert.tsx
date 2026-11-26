import { Link } from "@tanstack/react-router";
import { TriangleAlert } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { useDatabaseStorage } from "@/features/databases/hooks/use-database-storage";
import {
  almostFullAction,
  storageAlmostFull,
} from "@/features/databases/lib/storage";
import type { DatabaseDetailView } from "@/features/databases/types";

/**
 * Render's almost-full warning (w3/m656,
 * docs/render-artifacts/postgres-dashboard.md § Almost-full warning), at the
 * top of a database's Info, Plan and Recovery pages. It shows only once the
 * usage is read and Render's rule holds, so no pending frame draws it.
 */
export function DatabaseStorageAlert({
  database,
}: {
  database: DatabaseDetailView;
}) {
  const usage = useDatabaseStorage(database.id);
  if (!storageAlmostFull(usage)) return null;
  return <StorageAlmostFull database={database} />;
}

function StorageAlmostFull({ database }: { database: DatabaseDetailView }) {
  const { t } = useTranslations();
  const { instanceTypes, loading } = useDatabaseInstanceTypes();
  // The ending depends on the plan: wait for the catalog rather than switch.
  if (loading && instanceTypes.length === 0) return null;
  const action = almostFullAction(
    database,
    instanceTypes.find((it) => it.id === database.plan),
  );
  const [before, after] = aroundPlaceholder(
    t("databases.storageAlmostFull", { action: PLACEHOLDER_SENTINEL }),
  );
  return (
    <div
      role="alert"
      className="flex items-start gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm"
    >
      <TriangleAlert
        aria-hidden="true"
        className="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-400"
      />
      <p>
        {before}
        {action === "support" ? (
          t("databases.storageAlmostFullSupport")
        ) : (
          // Render's `/plan#storage`, or `/plan` for a fixed plan's picker.
          <Link
            to="/databases/$databaseId/plan"
            params={{ databaseId: database.id }}
            hash={action === "increaseStorage" ? "storage" : undefined}
            className="font-medium underline underline-offset-2"
          >
            {t(
              action === "increaseStorage"
                ? "databases.storageAlmostFullIncrease"
                : "databases.storageAlmostFullUpgrade",
            )}
          </Link>
        )}
        {after}
      </p>
    </div>
  );
}
