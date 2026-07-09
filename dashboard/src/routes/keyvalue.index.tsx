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
import { Alert, AlertDescription, AlertTitle } from "@/common/components/ui/alert.tsx";
import { Skeleton } from "@/common/components/ui/skeleton.tsx";
import { Button } from "@/common/components/ui/button.tsx";
import { Plus } from "lucide-react";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import { computeStats } from "@/features/keyvalue/lib/status";
import { formatRelativeAge } from "@/features/services/lib/format";
import { KeyValueStatusBadge } from "@/features/keyvalue/components/key-value-status-badge";
import { KeyValueRowActions } from "@/features/keyvalue/components/key-value-row-actions";
import type { KeyValueView } from "@/features/keyvalue/types";

export const Route = createFileRoute("/keyvalue/")({
  component: KeyValuePage,
  beforeLoad: requireAuth("/keyvalue"),
  head: () => ({
    meta: [{ title: "Key Value · bex dashboard" }],
  }),
});

export function KeyValuePage() {
  const { t } = useTranslations();
  const { keyValues, loading, error, refetch, startPolling, stopPolling } =
    useKeyValues();

  const stats = useMemo(() => computeStats(keyValues), [keyValues]);
  const kvStats = [
    { labelKey: "keyvalue.statTotal", value: stats.total },
    { labelKey: "keyvalue.statAvailable", value: stats.available },
    { labelKey: "keyvalue.statCreating", value: stats.creating },
  ] as const;

  // Poll the list while any store is still provisioning so a just-created row
  // converges to Available on its own; stop once everything is settled.
  useEffect(() => {
    if (stats.creating > 0) startPolling(3000);
    else stopPolling();
    return () => stopPolling();
  }, [stats.creating, startPolling, stopPolling]);

  // Loading/error only take over the page while there's nothing to show; a
  // transient poll error or background refetch must not blank an existing list.
  const showSkeleton = loading && keyValues.length === 0;
  const showError = !loading && error && keyValues.length === 0;
  const showEmpty = !loading && !error && keyValues.length === 0;

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            {kvStats.map((stat) => (
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
              <CardTitle>{t("keyvalue.cardTitle")}</CardTitle>
              <Button asChild>
                <Link to="/keyvalue/new">
                  <Plus />
                  {t("keyvalue.createButton")}
                </Link>
              </Button>
            </CardHeader>
            <CardContent>
              {showError ? (
                <Alert variant="destructive">
                  <AlertTitle>{t("keyvalue.errorTitle")}</AlertTitle>
                  <AlertDescription>{t("keyvalue.errorBody")}</AlertDescription>
                </Alert>
              ) : showEmpty ? (
                <div className="py-10 text-center">
                  <p className="font-medium">{t("keyvalue.emptyTitle")}</p>
                  <p className="text-sm text-muted-foreground">
                    {t("keyvalue.emptyBody")}
                  </p>
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("keyvalue.colName")}</TableHead>
                      <TableHead>{t("keyvalue.colStatus")}</TableHead>
                      <TableHead>{t("keyvalue.colPlan")}</TableHead>
                      <TableHead>{t("keyvalue.colVersion")}</TableHead>
                      <TableHead>{t("keyvalue.colCreated")}</TableHead>
                      <TableHead className="w-0 text-right">
                        <span className="sr-only">
                          {t("keyvalue.colActions")}
                        </span>
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {showSkeleton
                      ? Array.from({ length: 3 }).map((_, i) => (
                          <KeyValueSkeletonRow key={i} />
                        ))
                      : keyValues.map((kv) => (
                          <KeyValueRow
                            key={kv.id}
                            keyValue={kv}
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

function KeyValueRow({
  keyValue,
  onDeleted,
}: {
  keyValue: KeyValueView;
  onDeleted: (id: string) => void;
}) {
  return (
    <TableRow>
      <TableCell className="font-medium">
        <Link
          to="/keyvalue/$keyValueId"
          params={{ keyValueId: keyValue.id }}
          className="hover:underline"
        >
          {keyValue.name}
        </Link>
      </TableCell>
      <TableCell>
        <KeyValueStatusBadge status={keyValue.status} />
      </TableCell>
      <TableCell className="text-muted-foreground">
        {keyValue.plan ?? "—"}
      </TableCell>
      <TableCell className="text-muted-foreground">
        {keyValue.version ?? "—"}
      </TableCell>
      <TableCell className="tabular-nums text-muted-foreground">
        {formatRelativeAge(keyValue.createdAt)}
      </TableCell>
      <TableCell className="text-right">
        <KeyValueRowActions keyValue={keyValue} onDeleted={onDeleted} />
      </TableCell>
    </TableRow>
  );
}

function KeyValueSkeletonRow() {
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
      <TableCell>
        <Skeleton className="h-4 w-10" />
      </TableCell>
      <TableCell className="text-right">
        <Skeleton className="ml-auto size-8 rounded-md" />
      </TableCell>
    </TableRow>
  );
}
