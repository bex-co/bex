import { Link, createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { BlueprintStatusBadge } from "@/features/blueprints/components/blueprint-status-badge";
import { useBlueprints } from "@/features/blueprints/hooks/use-blueprints";
import { formatRelativeAge } from "@/features/services/lib/format";

export const Route = createFileRoute("/blueprints")({
  component: BlueprintsPage,
  beforeLoad: requireAuth("/blueprints"),
  head: () => ({
    meta: [{ title: "Blueprints — bex" }],
  }),
});

export function BlueprintsPage() {
  const { t } = useTranslations();
  const { blueprints, loading, error } = useBlueprints();

  const showSkeleton = loading && blueprints.length === 0;

  return (
    <DashboardLayout>
      <div className="flex items-center border-b px-4 py-4 sm:px-6">
        <h1 className="text-xl font-semibold">{t("blueprints.pageTitle")}</h1>
      </div>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          {error && blueprints.length === 0 ? (
            <div className="py-10 text-center">
              <p className="font-medium">{t("blueprints.errorTitle")}</p>
            </div>
          ) : showSkeleton ? (
            <Skeleton className="h-40 w-full" />
          ) : blueprints.length === 0 ? (
            <div className="py-10 text-center">
              <p className="font-medium">{t("blueprints.emptyTitle")}</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {t("blueprints.emptyBody")}
              </p>
            </div>
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>{t("blueprints.cardTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("blueprints.colName")}</TableHead>
                      <TableHead>{t("blueprints.colRepo")}</TableHead>
                      <TableHead>{t("blueprints.colBranch")}</TableHead>
                      <TableHead>{t("blueprints.colStatus")}</TableHead>
                      <TableHead>{t("blueprints.colUpdated")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {blueprints.map((bp) => (
                      <TableRow key={bp.id}>
                        <TableCell className="font-medium">
                          <Link
                            to="/blueprints/$blueprintId"
                            params={{ blueprintId: bp.id }}
                            className="hover:underline"
                          >
                            {bp.name}
                          </Link>
                        </TableCell>
                        <TableCell className="max-w-[200px] truncate text-sm text-muted-foreground">
                          {bp.repo}
                        </TableCell>
                        <TableCell className="text-sm">{bp.branch}</TableCell>
                        <TableCell>
                          <BlueprintStatusBadge status={bp.status} />
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {bp.updatedAt ? formatRelativeAge(bp.updatedAt) : "—"}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}
