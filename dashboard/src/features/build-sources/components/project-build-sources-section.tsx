import { Link } from "@tanstack/react-router";
import { Package, Plus } from "lucide-react";
import { LocalDateTime } from "@/common/components/local-time";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { regionLabel } from "@/common/lib/region-label";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { BuildSourceKindSummary } from "@/features/build-sources/components/build-source-kind-summary";
import { useBuildSourcesList } from "@/features/build-sources/hooks/use-build-sources-list";

export interface ProjectBuildSourcesSectionProps {
  projectId: string;
}

/**
 * Surfaces Build Sources scoped to `projectId` on the Project Overview page
 * (`features/projects/pages/project-page.tsx`), sharing the workspace Build
 * Sources Apollo
 * cache so creating a Build Source updates Project Overview immediately.
 */
export function ProjectBuildSourcesSection({
  projectId,
}: ProjectBuildSourcesSectionProps) {
  const { t } = useTranslations();
  const { canCreate } = useCapabilities();
  const { buildSources, loading, error, refetch } = useBuildSourcesList({
    projectId,
  });

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Package
            className="text-muted-foreground size-4"
            aria-hidden="true"
          />
          <h2 className="text-lg font-semibold">
            {t("buildSources.projectSectionTitle")}
          </h2>
          {!loading && buildSources.length > 0 ? (
            <Badge variant="secondary" className="tabular-nums">
              {buildSources.length}
            </Badge>
          ) : null}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {buildSources.length > 0 ? (
            <Button asChild variant="outline" size="sm">
              <Link to="/build-sources">{t("buildSources.viewAll")}</Link>
            </Button>
          ) : null}
          {canCreate ? (
            <Button asChild size="sm" variant="outline">
              <Link
                to="/build-sources/new"
                search={{ projectId }}
              >
                <Plus aria-hidden="true" />
                {t("buildSources.newButton")}
              </Link>
            </Button>
          ) : null}
        </div>
      </div>

      {loading ? (
        <Card className="py-4" aria-hidden="true">
          <CardContent className="space-y-3 px-4">
            <Skeleton className="h-4 w-48" />
            <Skeleton className="h-4 w-64" />
          </CardContent>
        </Card>
      ) : error && buildSources.length === 0 ? (
        <Card className="py-4">
          <CardContent className="flex flex-wrap items-center justify-between gap-2 px-4 text-sm">
            <span className="text-muted-foreground">
              {t("buildSources.projectLoadError")}
            </span>
            <Button size="sm" variant="outline" onClick={() => void refetch()}>
              {t("common.tryAgain")}
            </Button>
          </CardContent>
        </Card>
      ) : buildSources.length === 0 ? (
        <Card className="border-dashed py-6">
          <CardContent className="flex flex-col items-center gap-2 px-4 text-center">
            <p className="text-muted-foreground text-sm">
              {t("buildSources.projectEmptyDescription")}
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="hidden overflow-hidden rounded-lg border md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("buildSources.colName")}</TableHead>
                  <TableHead>{t("buildSources.colSource")}</TableHead>
                  <TableHead>{t("buildSources.colRegion")}</TableHead>
                  <TableHead>{t("buildSources.colLinkedServices")}</TableHead>
                  <TableHead className="text-right">
                    {t("buildSources.colUpdated")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {buildSources.map((source) => (
                  <TableRow key={source.id}>
                    <TableCell className="py-3 font-medium">
                      <Link
                        to="/build-sources/$buildSourceId"
                        params={{ buildSourceId: source.id }}
                        className="hover:underline"
                      >
                        {source.name}
                      </Link>
                    </TableCell>
                    <TableCell className="py-3 text-sm">
                      <BuildSourceKindSummary source={source} />
                    </TableCell>
                    <TableCell className="py-3 text-sm">
                      {source.git?.region
                        ? regionLabel(source.git.region, t)
                        : "—"}
                    </TableCell>
                    <TableCell className="py-3 text-sm tabular-nums">
                      {t("buildSources.linkedServicesCount", {
                        count: source.serviceLinks.length,
                      })}
                    </TableCell>
                    <TableCell className="text-muted-foreground py-3 text-right text-sm whitespace-nowrap">
                      <LocalDateTime value={source.updatedAt} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <div className="space-y-3 md:hidden">
            {buildSources.map((source) => (
              <Card key={source.id} className="gap-2 py-3.5">
                <CardContent className="space-y-2 px-4">
                  <div className="flex items-center justify-between gap-2">
                    <Link
                      to="/build-sources/$buildSourceId"
                      params={{ buildSourceId: source.id }}
                      className="truncate font-medium hover:underline"
                    >
                      {source.name}
                    </Link>
                  </div>
                  <BuildSourceKindSummary source={source} showRuntime={false} />
                  <div className="text-muted-foreground flex items-center justify-between gap-2 text-xs">
                    <span className="tabular-nums">
                      {t("buildSources.linkedServicesCount", {
                        count: source.serviceLinks.length,
                      })}
                    </span>
                    <LocalDateTime value={source.updatedAt} />
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </>
      )}
    </section>
  );
}
