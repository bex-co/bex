import { useEffect, useMemo } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table.tsx";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert.tsx";
import { Skeleton } from "@/common/components/ui/skeleton.tsx";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { computeStats } from "@/features/databases/lib/status";
import { formatRelativeAge } from "@/features/services/lib/format";
import { DatabaseStatusBadge } from "@/features/databases/components/database-status-badge";
import { DatabaseRowActions } from "@/features/databases/components/database-row-actions";
import { CreateDatabaseDialog } from "@/features/databases/components/create-database-dialog";
import type { DatabaseView } from "@/features/databases/types";

export const Route = createFileRoute("/databases/")({
  component: DatabasesPage,
  beforeLoad: requireAuth("/databases"),
  head: () => ({
    meta: [{ title: "Databases · bex dashboard" }],
  }),
});

export function DatabasesPage() {
  const { t } = useTranslations();
  const { databases, loading, error, refetch, startPolling, stopPolling } =
    useDatabases();

  const stats = useMemo(() => computeStats(databases), [databases]);
  const dbStats = [
    { labelKey: "databases.statTotal", value: stats.total },
    { labelKey: "databases.statAvailable", value: stats.available },
    { labelKey: "databases.statCreating", value: stats.creating },
  ] as const;

  // Poll the list while any database is still provisioning so a just-created row
  // converges to Available on its own; stop once everything is settled.
  useEffect(() => {
    if (stats.creating > 0) startPolling(3000);
    else stopPolling();
    return () => stopPolling();
  }, [stats.creating, startPolling, stopPolling]);

  // Loading/error only take over the page while there's nothing to show; a
  // transient poll error or background refetch must not blank an existing list.
  const showSkeleton = loading && databases.length === 0;
  const showError = !loading && error && databases.length === 0;
  const showEmpty = !loading && !error && databases.length === 0;

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            {dbStats.map((stat) => (
              <Card key={stat.labelKey}>
                <CardHeader>
                  <CardDescription>{t(stat.labelKey)}</CardDescription>
                  <CardTitle className="text-2xl tabular-nums">
                    {stat.value}
                  </CardTitle>
                </CardHeader>
              </Card>
            ))}
          </div>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between gap-2 space-y-0">
              <CardTitle>{t("databases.cardTitle")}</CardTitle>
              <CreateDatabaseDialog onCreated={() => void refetch()} />
            </CardHeader>
            <CardContent>
              {showError ? (
                <Alert variant="destructive">
                  <AlertTitle>{t("databases.errorTitle")}</AlertTitle>
                  <AlertDescription>
                    {t("databases.errorBody")}
                  </AlertDescription>
                </Alert>
              ) : showEmpty ? (
                <div className="py-10 text-center">
                  <p className="font-medium">{t("databases.emptyTitle")}</p>
                  <p className="text-sm text-muted-foreground">
                    {t("databases.emptyBody")}
                  </p>
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("databases.colName")}</TableHead>
                      <TableHead>{t("databases.colStatus")}</TableHead>
                      <TableHead>{t("databases.colPlan")}</TableHead>
                      <TableHead>{t("databases.colVersion")}</TableHead>
                      <TableHead className="text-right tabular-nums">
                        {t("databases.colStorage")}
                      </TableHead>
                      <TableHead>{t("databases.colCreated")}</TableHead>
                      <TableHead className="w-0 text-right">
                        <span className="sr-only">
                          {t("databases.colActions")}
                        </span>
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {showSkeleton
                      ? Array.from({ length: 3 }).map((_, i) => (
                          <DatabaseSkeletonRow key={i} />
                        ))
                      : databases.map((db) => (
                          <DatabaseRow
                            key={db.id}
                            database={db}
                            onDeleted={() => void refetch()}
                          />
                        ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </DashboardLayout>
  );
}

function DatabaseRow({
  database,
  onDeleted,
}: {
  database: DatabaseView;
  onDeleted: (id: string) => void;
}) {
  return (
    <TableRow>
      <TableCell className="font-medium">
        <Link
          to="/databases/$databaseId"
          params={{ databaseId: database.id }}
          className="hover:underline"
        >
          {database.name}
        </Link>
      </TableCell>
      <TableCell>
        <DatabaseStatusBadge status={database.status} />
      </TableCell>
      <TableCell className="text-muted-foreground">
        {database.plan ?? "—"}
      </TableCell>
      <TableCell className="text-muted-foreground">
        {database.version ?? "—"}
      </TableCell>
      <TableCell className="text-right tabular-nums text-muted-foreground">
        {database.diskSizeGB ? `${database.diskSizeGB} GB` : "—"}
      </TableCell>
      <TableCell className="tabular-nums text-muted-foreground">
        {formatRelativeAge(database.createdAt)}
      </TableCell>
      <TableCell className="text-right">
        <DatabaseRowActions database={database} onDeleted={onDeleted} />
      </TableCell>
    </TableRow>
  );
}

function DatabaseSkeletonRow() {
  return (
    <TableRow>
      <TableCell>
        <Skeleton className="h-4 w-32" />
      </TableCell>
      <TableCell>
        <Skeleton className="h-5 w-16 rounded-md" />
      </TableCell>
      <TableCell>
        <Skeleton className="h-4 w-20" />
      </TableCell>
      <TableCell>
        <Skeleton className="h-4 w-10" />
      </TableCell>
      <TableCell className="text-right">
        <Skeleton className="ml-auto h-4 w-12" />
      </TableCell>
      <TableCell>
        <Skeleton className="h-4 w-10" />
      </TableCell>
      <TableCell className="text-right">
        <Skeleton className="ml-auto size-8 rounded-md" />
      </TableCell>
    </TableRow>
  );
}
