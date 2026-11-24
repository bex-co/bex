import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  FolderKanban,
  Package,
  Plus,
  Search,
  X,
} from "lucide-react";
import { isUnauthenticatedError } from "@/common/apollo/auth-error-link";
import { CopyButton } from "@/common/components/copy-button";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { LocalDateTime } from "@/common/components/local-time";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useTranslations } from "@/common/hooks/use-translations";
import { regionLabel } from "@/common/lib/region-label";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { BuildSourceKindSummary } from "@/features/build-sources/components/build-source-kind-summary";
import { BuildSourcesListContentSkeleton } from "@/features/build-sources/components/build-source-skeletons";
import { useBuildSourcesList } from "@/features/build-sources/hooks/use-build-sources-list";
import type {
  BuildSourceKind,
  BuildSourceView,
} from "@/features/build-sources/types";

type KindFilter = "all" | BuildSourceKind;

export interface BuildSourcesListPageProps {
  initialQuery?: string;
  initialKind?: BuildSourceKind;
  onFilterChange?: (next: { q?: string; kind?: BuildSourceKind }) => void;
}

export function BuildSourcesListPage({
  initialQuery = "",
  initialKind,
  onFilterChange,
}: BuildSourcesListPageProps) {
  const hydrated = useIsHydrated();
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const { canCreate } = useCapabilities();
  const { buildSources, loading, error, refetch } = useBuildSourcesList();
  const { projects } = useProjects({ poll: false });

  const [filterState, setFilterState] = useState<{
    workspaceId: string | null;
    query: string;
    kind: KindFilter;
  }>({
    workspaceId: currentWorkspaceId,
    query: initialQuery,
    kind: initialKind ?? "all",
  });

  const query =
    filterState.workspaceId === currentWorkspaceId ? filterState.query : "";
  const kind: KindFilter =
    filterState.workspaceId === currentWorkspaceId ? filterState.kind : "all";

  function updateQuery(nextQuery: string) {
    setFilterState({
      workspaceId: currentWorkspaceId,
      query: nextQuery,
      kind,
    });
    onFilterChange?.({
      q: nextQuery.trim() || undefined,
      kind: kind === "all" ? undefined : kind,
    });
  }

  function updateKind(nextKind: KindFilter) {
    setFilterState({
      workspaceId: currentWorkspaceId,
      query,
      kind: nextKind,
    });
    onFilterChange?.({
      q: query.trim() || undefined,
      kind: nextKind === "all" ? undefined : nextKind,
    });
  }

  function resetFilters() {
    setFilterState({
      workspaceId: currentWorkspaceId,
      query: "",
      kind: "all",
    });
    onFilterChange?.({ q: undefined, kind: undefined });
  }

  const projectNames = useMemo(
    () => new Map(projects.map((p) => [p.id, p.name])),
    [projects],
  );

  const normalizedQuery = query.trim().toLocaleLowerCase();
  const visibleSources = useMemo(
    () =>
      buildSources.filter((item) => {
        if (kind !== "all" && item.kind !== kind) return false;
        if (!normalizedQuery) return true;
        const projectName = item.projectId
          ? (projectNames.get(item.projectId) ?? item.projectId)
          : "";
        const haystack = [
          item.name,
          item.id,
          item.git?.repoUrl,
          item.git?.branch,
          item.git?.runtime,
          item.git?.region,
          item.image?.imageUrl,
          projectName,
        ]
          .filter(Boolean)
          .join(" ")
          .toLocaleLowerCase();
        return haystack.includes(normalizedQuery);
      }),
    [buildSources, kind, normalizedQuery, projectNames],
  );

  const hasFilters = query.trim() !== "" || kind !== "all";
  const unauthenticated = error != null && isUnauthenticatedError(error);

  return (
    <DashboardLayout>
      <PageFrame
        header={
          <PageHeader
            title={t("buildSources.pageTitle")}
            description={t("buildSources.pageDescription")}
            actions={
              canCreate ? (
                <Button asChild size="sm">
                  <Link to="/build-sources/new">
                    <Plus aria-hidden="true" />
                    {t("buildSources.newButton")}
                  </Link>
                </Button>
              ) : null
            }
          />
        }
      >
        {loading ? (
          <BuildSourcesListContentSkeleton />
        ) : unauthenticated && buildSources.length === 0 ? (
          <ResourceLoadError variant="unauthenticated" />
        ) : error && buildSources.length === 0 ? (
          <ResourceLoadError onRetry={() => void refetch()} />
        ) : buildSources.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed px-4 py-12 text-center">
            <Package
              className="text-muted-foreground size-8"
              aria-hidden="true"
            />
            <p className="text-sm font-medium">
              {t("buildSources.emptyTitle")}
            </p>
            <p className="text-muted-foreground max-w-md text-xs">
              {t("buildSources.emptyDescription")}
            </p>
            {canCreate ? (
              <Button asChild size="sm" variant="outline" className="mt-2">
                <Link to="/build-sources/new">
                  <Plus aria-hidden="true" />
                  {t("buildSources.newButton")}
                </Link>
              </Button>
            ) : null}
          </div>
        ) : (
          <div className="space-y-4">
            {error ? (
              <Alert variant="destructive">
                <AlertTriangle className="size-4" aria-hidden="true" />
                <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
                  <span>{t("buildSources.refreshError")}</span>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void refetch()}
                  >
                    {t("common.tryAgain")}
                  </Button>
                </AlertDescription>
              </Alert>
            ) : null}

            {/* Search + source-kind filter toolbar */}
            <div
              data-region="filters"
              className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="flex flex-1 items-center gap-2">
                <div className="relative w-full sm:max-w-md">
                  <Search
                    className="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
                    aria-hidden="true"
                  />
                  <Input
                    value={query}
                    disabled={!hydrated}
                    onChange={(event) => updateQuery(event.target.value)}
                    placeholder={t("buildSources.searchPlaceholder")}
                    aria-label={t("buildSources.searchLabel")}
                    className="h-11 pl-9 sm:h-9"
                  />
                </div>
                {hasFilters ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={resetFilters}
                    className="shrink-0"
                  >
                    <X className="size-3.5" aria-hidden="true" />
                    {t("buildSources.resetFilters")}
                  </Button>
                ) : null}
              </div>

              <div
                role="group"
                aria-label={t("buildSources.filterKindLabel")}
                className="flex flex-wrap items-center gap-1.5"
              >
                {(
                  [
                    ["all", "buildSources.filterAll"],
                    ["git", "buildSources.kindGit"],
                    ["image", "buildSources.kindImage"],
                  ] as const
                ).map(([value, labelKey]) => (
                  <Button
                    key={value}
                    type="button"
                    size="sm"
                    variant={kind === value ? "default" : "outline"}
                    aria-pressed={kind === value}
                    onClick={() => updateKind(value)}
                  >
                    {t(labelKey)}
                  </Button>
                ))}
              </div>
            </div>

            <div data-region="build-sources-list">
              {visibleSources.length === 0 ? (
                <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed px-4 py-12 text-center">
                  <Search
                    className="text-muted-foreground size-7"
                    aria-hidden="true"
                  />
                  <p className="text-sm font-medium">
                    {t("buildSources.noMatchesTitle")}
                  </p>
                  <p className="text-muted-foreground text-xs">
                    {t("buildSources.noMatchesDescription")}
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="mt-1"
                    onClick={resetFilters}
                  >
                    {t("buildSources.resetFilters")}
                  </Button>
                </div>
              ) : (
                <div className="space-y-4">
                  {/* Desktop table (md+) */}
                  <div className="hidden overflow-hidden rounded-lg border md:block">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-[22%]">
                            {t("buildSources.colName")}
                          </TableHead>
                          <TableHead className="w-[28%]">
                            {t("buildSources.colSource")}
                          </TableHead>
                          <TableHead className="w-[13%]">
                            {t("buildSources.colRegion")}
                          </TableHead>
                          <TableHead className="w-[13%]">
                            {t("buildSources.colLinkedServices")}
                          </TableHead>
                          <TableHead className="w-[12%]">
                            {t("buildSources.colProject")}
                          </TableHead>
                          <TableHead className="w-[12%] text-right">
                            {t("buildSources.colUpdated")}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {visibleSources.map((source) => (
                          <BuildSourceTableRow
                            key={source.id}
                            source={source}
                            projectName={
                              source.projectId
                                ? (projectNames.get(source.projectId) ??
                                  source.projectId)
                                : null
                            }
                          />
                        ))}
                      </TableBody>
                    </Table>
                  </div>

                  {/* Narrow-mobile stacked cards (< md, 390px) */}
                  <div className="space-y-3 md:hidden">
                    {visibleSources.map((source) => (
                      <BuildSourceMobileCard
                        key={source.id}
                        source={source}
                        projectName={
                          source.projectId
                            ? (projectNames.get(source.projectId) ??
                              source.projectId)
                            : null
                        }
                      />
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </PageFrame>
    </DashboardLayout>
  );
}

function BuildSourceTableRow({
  source,
  projectName,
}: {
  source: BuildSourceView;
  projectName: string | null;
}) {
  const { t } = useTranslations();
  const region = source.git?.region ? regionLabel(source.git.region, t) : "—";

  return (
    <TableRow>
      <TableCell className="py-3.5">
        <div className="min-w-0 space-y-0.5">
          <Link
            to="/build-sources/$buildSourceId"
            params={{ buildSourceId: source.id }}
            className="block truncate font-medium hover:underline"
          >
            {source.name}
          </Link>
          <div className="text-muted-foreground flex items-center gap-1 font-mono text-xs">
            <span className="truncate">{source.id}</span>
            <CopyButton
              value={source.id}
              label={t("buildSources.copyId")}
              successText={t("buildSources.copied")}
              errorText={t("buildSources.copyError")}
            />
          </div>
        </div>
      </TableCell>
      <TableCell className="py-3.5">
        <BuildSourceKindSummary source={source} />
      </TableCell>
      <TableCell className="py-3.5 text-sm">{region}</TableCell>
      <TableCell className="py-3.5 text-sm tabular-nums">
        {t("buildSources.linkedServicesCount", {
          count: source.serviceLinks.length,
        })}
      </TableCell>
      <TableCell className="py-3.5">
        {source.projectId && projectName ? (
          <Badge variant="outline" asChild className="max-w-full gap-1">
            <Link
              to="/project/$projectId"
              params={{ projectId: source.projectId }}
              className="truncate"
            >
              <FolderKanban className="size-3 shrink-0" aria-hidden="true" />
              <span className="truncate">{projectName}</span>
            </Link>
          </Badge>
        ) : (
          <span className="text-muted-foreground text-xs">
            {t("buildSources.workspaceScope")}
          </span>
        )}
      </TableCell>
      <TableCell className="text-muted-foreground py-3.5 text-right text-sm whitespace-nowrap">
        <LocalDateTime value={source.updatedAt} />
      </TableCell>
    </TableRow>
  );
}

function BuildSourceMobileCard({
  source,
  projectName,
}: {
  source: BuildSourceView;
  projectName: string | null;
}) {
  const { t } = useTranslations();
  const region = source.git?.region ? regionLabel(source.git.region, t) : null;

  return (
    <Card className="gap-3 py-4">
      <CardContent className="space-y-3 px-4">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 space-y-0.5">
            <Link
              to="/build-sources/$buildSourceId"
              params={{ buildSourceId: source.id }}
              className="block truncate font-medium hover:underline"
            >
              {source.name}
            </Link>
            <div className="text-muted-foreground flex items-center gap-1 font-mono text-xs">
              <span className="truncate">{source.id}</span>
              <CopyButton
                value={source.id}
                label={t("buildSources.copyId")}
                successText={t("buildSources.copied")}
                errorText={t("buildSources.copyError")}
              />
            </div>
          </div>
          {source.projectId && projectName ? (
            <Badge variant="outline" asChild className="max-w-36 shrink-0 gap-1">
              <Link
                to="/project/$projectId"
                params={{ projectId: source.projectId }}
                className="truncate"
              >
                <FolderKanban className="size-3 shrink-0" aria-hidden="true" />
                <span className="truncate">{projectName}</span>
              </Link>
            </Badge>
          ) : null}
        </div>

        <BuildSourceKindSummary source={source} />

        <div className="text-muted-foreground flex flex-wrap items-center justify-between gap-2 border-t pt-2.5 text-xs">
          <span className="tabular-nums">
            {t("buildSources.linkedServicesCount", {
              count: source.serviceLinks.length,
            })}
            {region ? ` · ${region}` : ""}
          </span>
          <LocalDateTime value={source.updatedAt} />
        </div>
      </CardContent>
    </Card>
  );
}
