import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  ArrowUpCircle,
  CircleAlert,
  Info,
  Loader2,
  TriangleAlert,
  X,
} from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useAcknowledgeDatabaseUpgrade } from "@/features/databases/hooks/use-acknowledge-database-upgrade";
import {
  dismissUpgradeAvailable,
  isUpgradeClone,
  unacknowledgedFailure,
  upgradeAvailable,
  upgradeRunning,
  useUpgradeAvailableDismissed,
  useUpgradeClonePair,
  versionUpgradeEligible,
} from "@/features/databases/lib/version-upgrade";
import type { DatabaseDetailView } from "@/features/databases/types";

// What a database's other pages say about its version upgrade (w3/m676), as
// Render's do: Info's version row links the guided page, Info announces a
// newer major and a failed upgrade, and a clone made for a test upgrade says
// so on every page.

/**
 * Info's Version value: the major, then Render's link to the guided page —
 * Upgrade version while a newer major exists, View upgrade while one runs,
 * else Review version details. A free database has no upgrade page, as on
 * Render, so its row is the version alone.
 */
export function DatabaseVersionValue({
  database,
}: {
  database: DatabaseDetailView;
}) {
  const { t } = useTranslations();
  const version = database.version
    ? t("databases.versionLabel", { version: database.version })
    : "—";
  if (!versionUpgradeEligible(database)) return <span>{version}</span>;
  const running = upgradeRunning(database.lastUpgradeRun);
  const available = upgradeAvailable(database);
  return (
    <span className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1">
      <span>{version}</span>
      <Link
        to="/databases/$databaseId/version-upgrade"
        params={{ databaseId: database.id }}
        className="inline-flex items-center gap-1 whitespace-nowrap font-medium text-primary underline-offset-2 hover:underline"
      >
        {running || available ? (
          <ArrowUpCircle aria-hidden="true" className="size-4" />
        ) : null}
        {running
          ? t("databases.versionUpgradeViewLink")
          : available
            ? t("databases.versionUpgradeLink")
            : t("databases.versionUpgradeReviewLink")}
        {running || available ? null : (
          <ArrowRight aria-hidden="true" className="size-4" />
        )}
      </Link>
    </span>
  );
}

/**
 * Render's "Upgrade failed." banner, on Info and on the guided page: a
 * failed run nobody dismissed. Dismiss error acknowledges the run.
 */
export function DatabaseUpgradeFailedBanner({
  database,
}: {
  database: DatabaseDetailView;
}) {
  const run = database.lastUpgradeRun;
  if (!unacknowledgedFailure(run)) return null;
  return <UpgradeFailedBanner databaseId={database.id} runId={run.runId} />;
}

function UpgradeFailedBanner({
  databaseId,
  runId,
}: {
  databaseId: string;
  runId: string;
}) {
  const { t } = useTranslations();
  const { acknowledge, busy } = useAcknowledgeDatabaseUpgrade();
  const { denied, reason } = useCapabilityGate("canCreate");
  return (
    <div
      role="alert"
      className="flex flex-wrap items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm"
    >
      <CircleAlert
        aria-hidden="true"
        className="mt-0.5 size-4 shrink-0 text-destructive"
      />
      <div className="min-w-0 flex-1 space-y-0.5">
        <p className="font-medium">
          {t("databases.versionUpgradeFailedBannerTitle")}
        </p>
        <p>{t("databases.versionUpgradeFailedBody")}</p>
      </div>
      <PermissionTooltip reason={reason}>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={busy || denied}
          onClick={() => void acknowledge(databaseId, runId)}
        >
          {busy ? <Loader2 className="animate-spin" /> : null}
          {t("databases.versionUpgradeDismiss")}
        </Button>
      </PermissionTooltip>
    </div>
  );
}

/**
 * Render's Info banner for a database on an older major: "PostgreSQL 18 is
 * available.", with View upgrade options, dismissed for good in this browser.
 * It stays away while an upgrade runs.
 */
export function DatabaseUpgradeAvailableBanner({
  database,
}: {
  database: DatabaseDetailView;
}) {
  const { t } = useTranslations();
  const dismissed = useUpgradeAvailableDismissed(database.id);
  if (
    dismissed ||
    !upgradeAvailable(database) ||
    upgradeRunning(database.lastUpgradeRun)
  ) {
    return null;
  }
  return (
    <div
      role="status"
      className="flex flex-wrap items-start gap-3 rounded-lg border border-sky-500/40 bg-sky-500/10 px-4 py-3 text-sm"
    >
      <Info
        aria-hidden="true"
        className="mt-0.5 size-4 shrink-0 text-sky-600 dark:text-sky-400"
      />
      <div className="min-w-0 flex-1 space-y-0.5">
        <p className="font-medium">
          {t("databases.versionUpgradeAvailableTitle", {
            version: database.versionUpgradeAvailable ?? "",
          })}
        </p>
        <p>{t("databases.versionUpgradeAvailableBody")}</p>
      </div>
      <div className="flex items-center gap-1">
        <Button asChild size="sm" variant="outline">
          <Link
            to="/databases/$databaseId/version-upgrade"
            params={{ databaseId: database.id }}
          >
            {t("databases.versionUpgradeViewOptions")}
          </Link>
        </Button>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-8"
          aria-label={t("databases.versionUpgradeAvailableDismiss")}
          onClick={() => dismissUpgradeAvailable(database.id)}
        >
          <X aria-hidden="true" />
        </Button>
      </div>
    </div>
  );
}

/**
 * Render's banner on a clone made for a test upgrade, above every page of
 * the clone: which database it copies, that it is billed like any other,
 * Delete this clone and Go to original database. As on Render, the source
 * is known only in the browser that made the clone.
 */
export function DatabaseUpgradeCloneBanner({
  database,
}: {
  database: DatabaseDetailView;
}) {
  const { t } = useTranslations();
  const source = useUpgradeClonePair(database.id);
  if (!source || !isUpgradeClone(database.name)) return null;
  return (
    <div
      role="status"
      className="flex flex-wrap items-start gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm"
    >
      <TriangleAlert
        aria-hidden="true"
        className="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-400"
      />
      <div className="min-w-0 flex-1 space-y-2">
        <div className="space-y-0.5">
          <p className="font-medium">
            {t("databases.versionUpgradeCloneOf", { name: source.name })}
          </p>
          <p>{t("databases.versionUpgradeCloneOfBody")}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button asChild size="sm">
            <Link
              to="/databases/$databaseId"
              params={{ databaseId: database.id }}
              hash="danger-zone"
            >
              {t("databases.versionUpgradeDeleteClone")}
            </Link>
          </Button>
          <Button asChild size="sm" variant="outline">
            <Link
              to="/databases/$databaseId/version-upgrade"
              params={{ databaseId: source.id }}
            >
              {t("databases.versionUpgradeGoToOriginal")}
              <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
        </div>
      </div>
    </div>
  );
}
