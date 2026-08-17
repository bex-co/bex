import { useState } from "react";
import {
  Link,
  Outlet,
  createFileRoute,
  useNavigate,
  useRouterState,
} from "@tanstack/react-router";
import { ListPageSkeleton } from "@/common/components/detail-skeletons";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  translatedTitleHead,
  titleLoaderFetchPolicy,
} from "@/common/lib/document-head";
import { prefetchInParallel } from "@/common/lib/prefetch";
import { BlueprintsDocument } from "@/features/blueprints/api/operations";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { GenerateBlueprintDialog } from "@/features/blueprints/components/generate-blueprint-dialog";
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
  staticData: { chrome: true },
  component: BlueprintsPage,
  pendingComponent: ListPageSkeleton,
  beforeLoad: requireAuth(),
  // Prefetch the list on hover-intent so it renders warm on mount (w9/m68);
  // `useBlueprints` is cache-first, so it reads this loader's result. The loader
  // fetches fresh on every entry (network-only), so cache-first carries no
  // staleness.
  loader: ({ context, cause }) => {
    const ownerId = context.workspaceId;
    if (ownerId == null) return;
    return prefetchInParallel([
      () =>
        context.client.query({
          query: BlueprintsDocument,
          variables: { ownerId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
    ]);
  },
  head: ({ match }) => translatedTitleHead("blueprints.pageTitle", match),
});

export function BlueprintsPage() {
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });

  if (pathname !== "/blueprints") return <Outlet />;

  return <BlueprintsListPage />;
}

function BlueprintsListPage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { blueprints, loading, error } = useBlueprints();
  const [generating, setGenerating] = useState(false);

  const showSkeleton = loading && blueprints.length === 0;

  return (
    <DashboardLayout>
      <div className="flex items-center justify-between border-b px-4 py-4 sm:px-6">
        <h1 className="text-xl font-semibold">{t("blueprints.pageTitle")}</h1>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => setGenerating(true)}
          >
            {t("blueprints.generateButton")}
          </Button>
          <Button
            size="sm"
            onClick={() => void navigate({ to: "/blueprints/new" })}
          >
            {t("blueprints.createButton")}
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          {error && blueprints.length === 0 ? (
            <div className="py-8 text-center">
              <p className="font-medium">{t("blueprints.errorTitle")}</p>
            </div>
          ) : showSkeleton ? (
            <Skeleton className="h-40 w-full" />
          ) : blueprints.length === 0 ? (
            <div className="py-8 text-center">
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
      {generating ? (
        <GenerateBlueprintDialog open onOpenChange={setGenerating} />
      ) : null}
    </DashboardLayout>
  );
}
