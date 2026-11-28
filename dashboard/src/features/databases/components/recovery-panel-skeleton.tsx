import { DatabaseBackup, Download } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The Recovery panel's content while its first read is in flight, shaped like
 * what it resolves to (dashboard/AGENTS.md § Navigation pending states). A
 * database with backups gets the window's two bounds, the plan's retention
 * line (w7/m453), the Restore and Export actions, one backup row, the export
 * scope line when the panel names a database, and the export list's empty
 * line (exports are on demand and expire, so most databases have none). One
 * without backups gets the line its disabled state shows.
 *
 * Static copy is the ready panel's own text laid out invisibly, so it wraps
 * as the ready text does at every width and in every language (w3/121): the
 * actions, the list headings, the scope line (it names `exportScope`) and the
 * empty and disabled lines. Only the data the read decides is a bar: the
 * bounds and the retention line, measured in English against the ready panel
 * at 1440 px and 390 px (below `sm` each field label and the retention line
 * take two lines), and the backup row, one line at every width.
 */
export function RecoveryContentSkeleton({
  backupsEnabled,
  exportScope,
}: {
  backupsEnabled: boolean;
  /** The database an export dumps, which the ready panel's scope line
   *  names; null where it names none. */
  exportScope: string | null;
}) {
  const { t } = useTranslations();
  if (!backupsEnabled) {
    return (
      <div data-testid="recovery-skeleton">
        <ReservedLine>{t("databases.recoveryDisabled")}</ReservedLine>
      </div>
    );
  }
  return (
    <div data-testid="recovery-skeleton" className="space-y-4">
      <div className="grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2">
        {[0, 1].map((i) => (
          <div key={i} className="flex justify-between gap-4 border-b pb-2">
            <Skeleton className="h-10 w-28 sm:h-5" />
            <Skeleton className="h-5 w-36" />
          </div>
        ))}
      </div>
      <Skeleton
        data-testid="recovery-skeleton-retention"
        className="h-10 w-3/4 sm:h-5"
      />
      <div className="flex flex-wrap gap-2">
        <ReservedButton variant="default">
          <DatabaseBackup />
          {t("databases.recoveryRestore")}
        </ReservedButton>
        <ReservedButton variant="outline">
          <Download />
          {t("databases.recoveryCreateExport")}
        </ReservedButton>
      </div>
      <SkeletonList title={t("databases.recoveryBackups")}>
        {/* The row's border and padding around its 22px status badge. */}
        <Skeleton data-testid="recovery-skeleton-backup" className="h-10" />
      </SkeletonList>
      {exportScope ? (
        <ReservedLine>
          {t("databases.recoveryExportScope", { database: exportScope })}
        </ReservedLine>
      ) : null}
      <SkeletonList title={t("databases.recoveryExports")}>
        <ReservedLine fit>{t("databases.recoveryNoExports")}</ReservedLine>
      </SkeletonList>
    </div>
  );
}

/** A ready muted `text-sm` paragraph's footprint, wrapping as its copy does;
 *  `fit` hugs a one-line text instead of spanning the card. */
function ReservedLine({
  fit = false,
  children,
}: {
  fit?: boolean;
  children: React.ReactNode;
}) {
  return (
    <p className={fit ? "relative w-fit text-sm" : "relative text-sm"}>
      <ReservedText>{children}</ReservedText>
    </p>
  );
}

/** A ready list's `text-sm font-medium` heading over its rows. */
function SkeletonList({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <p className="relative w-fit text-sm font-medium">
        <ReservedText>{title}</ReservedText>
      </p>
      {children}
    </div>
  );
}

/**
 * The whole Recovery card while its code loads (the route's Suspense
 * fallback, and the page's pending frame): the ready title and description,
 * whose text wraps as the ready header's does (three lines at 390 px), then
 * the same content skeleton.
 */
export function RecoveryPanelSkeleton(props: {
  backupsEnabled: boolean;
  exportScope: string | null;
}) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <CardTitle as="div" className="relative w-fit">
          <ReservedText>{t("databases.recoveryTitle")}</ReservedText>
        </CardTitle>
        <CardDescription className="relative">
          <ReservedText>{t("databases.recoveryDescription")}</ReservedText>
        </CardDescription>
      </CardHeader>
      <CardContent>
        <RecoveryContentSkeleton {...props} />
      </CardContent>
    </Card>
  );
}
