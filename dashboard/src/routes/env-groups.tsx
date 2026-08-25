import { useState } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { ListPageSkeleton } from "@/common/components/detail-skeletons";
import { AlertTriangle, Layers3, Search, X } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  translatedTitleHead,
  titleLoaderFetchPolicy,
} from "@/common/lib/document-head";
import { prefetchInParallel } from "@/common/lib/prefetch";
import {
  EnvGroupsDocument,
  EnvGroupScopeIndexDocument,
  ServicesDocument,
} from "@/graphql/definitions";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { NewEnvGroupDialog } from "@/features/env-groups/components/new-env-group-dialog";
import {
  classifyEnvGroupError,
  useEnvGroups,
} from "@/features/env-groups/hooks/use-env-groups";
import { useServices } from "@/features/services/hooks/use-services";
import { useEnvGroupScopeIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { formatDateTime } from "@/common/lib/format";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export const Route = createFileRoute("/env-groups")({
  staticData: { chrome: true },
  component: EnvGroupsPage,
  pendingComponent: ListPageSkeleton,
  beforeLoad: requireAuth(),
  // Warm the list + the scope index + services the create dialog needs so
  // hover-intent navigation skips the post-click skeleton waterfall.
  loader: ({ context, cause }) => {
    const ownerId = context.workspaceId;
    if (ownerId == null) return;
    const fetchPolicy = titleLoaderFetchPolicy(cause);
    return prefetchInParallel([
      () =>
        context.client.query({
          query: EnvGroupsDocument,
          variables: { ownerId },
          fetchPolicy,
          errorPolicy: "all",
        }),
      () =>
        context.client.query({
          query: ServicesDocument,
          variables: { ownerId },
          fetchPolicy,
          errorPolicy: "all",
        }),
      () =>
        context.client.query({
          query: EnvGroupScopeIndexDocument,
          variables: { ownerId },
          fetchPolicy,
          errorPolicy: "all",
        }),
    ]);
  },
  head: ({ match }) => translatedTitleHead("envGroups.pageTitle", match),
});

export function EnvGroupsPage() {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const navigate = useNavigate();
  const { groups, loading, error, refetch } = useEnvGroups();
  const { services, loading: servicesLoading } = useServices();
  const scope = useEnvGroupScopeIndex();
  const [searchState, setSearchState] = useState({
    workspaceId: currentWorkspaceId,
    value: "",
  });
  const search =
    searchState.workspaceId === currentWorkspaceId ? searchState.value : "";
  const setSearch = (value: string) =>
    setSearchState({ workspaceId: currentWorkspaceId, value });
  const errorKind = classifyEnvGroupError(error);
  const initialLoading = loading && groups.length === 0;
  const normalizedSearch = search.trim().toLocaleLowerCase();
  const visibleGroups = normalizedSearch
    ? groups.filter((group) =>
        group.name.toLocaleLowerCase().includes(normalizedSearch),
      )
    : groups;
  const environmentLabel = (environmentId: string | null) => {
    if (!environmentId) return t("envGroups.workspaceScope");
    return (
      scope.byId.get(environmentId)?.name ??
      t("envGroups.unknownEnvironment", { id: environmentId })
    );
  };

  return (
    <DashboardLayout>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6">
        <div>
          <h1 className="text-xl font-semibold">{t("envGroups.pageTitle")}</h1>
          <p className="text-sm text-muted-foreground">
            {t("envGroups.pageDescription")}
          </p>
        </div>
        <NewEnvGroupDialog
          refetch={refetch}
          services={services}
          servicesLoading={servicesLoading}
          onCreated={(groupId) =>
            void navigate({
              to: "/env-groups/$groupId",
              params: { groupId },
            })
          }
        />
      </div>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-5xl space-y-6">
          {errorKind && groups.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-12 text-center">
              <AlertTriangle className="size-8 text-destructive" />
              <p className="font-medium">{t(`envGroups.${errorKind}Title`)}</p>
              <p className="text-sm text-muted-foreground">
                {t(`envGroups.${errorKind}Body`)}
              </p>
            </div>
          ) : initialLoading ? (
            <div className="grid gap-4 md:grid-cols-2">
              <Skeleton className="h-44" />
              <Skeleton className="h-44" />
            </div>
          ) : groups.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-12 text-center">
              <Layers3 className="size-8 text-muted-foreground" />
              <p className="font-medium">{t("envGroups.emptyTitle")}</p>
              <p className="max-w-md text-sm text-muted-foreground">
                {t("envGroups.emptyBody")}
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                <div className="relative max-w-md flex-1">
                  <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    placeholder={t("envGroups.searchPlaceholder")}
                    aria-label={t("envGroups.searchLabel")}
                    className="pl-9"
                  />
                </div>
                {search ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setSearch("")}
                  >
                    <X /> {t("envGroups.resetSearch")}
                  </Button>
                ) : null}
              </div>
              {visibleGroups.length === 0 ? (
                <div className="flex flex-col items-center gap-2 py-12 text-center">
                  <Search className="size-8 text-muted-foreground" />
                  <p className="font-medium">{t("envGroups.noMatchesTitle")}</p>
                  <p className="text-sm text-muted-foreground">
                    {t("envGroups.noMatchesBody")}
                  </p>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setSearch("")}
                  >
                    {t("envGroups.resetSearch")}
                  </Button>
                </div>
              ) : (
                <div className="rounded-md border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("envGroups.colName")}</TableHead>
                        <TableHead>{t("envGroups.colEnvironment")}</TableHead>
                        <TableHead className="text-right">
                          {t("envGroups.colEnvVars")}
                        </TableHead>
                        <TableHead className="text-right">
                          {t("envGroups.colSecretFiles")}
                        </TableHead>
                        <TableHead className="text-right">
                          {t("envGroups.colUpdated")}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {visibleGroups.map((group) => (
                        <TableRow key={group.id}>
                          <TableCell className="min-w-48">
                            <Link
                              to="/env-groups/$groupId"
                              params={{ groupId: group.id }}
                              className="font-medium hover:underline"
                            >
                              {group.name}
                            </Link>
                            <p className="max-w-64 truncate font-mono text-xs text-muted-foreground">
                              {group.id} ·{" "}
                              {t("envGroups.serviceCount", {
                                count: group.serviceLinks.length,
                              })}
                            </p>
                          </TableCell>
                          <TableCell className="min-w-40">
                            {environmentLabel(group.environmentId)}
                          </TableCell>
                          <TableCell className="text-right tabular-nums">
                            {group.envVarKeys.length}
                          </TableCell>
                          <TableCell className="text-right tabular-nums">
                            {group.secretFileNames.length}
                          </TableCell>
                          {/* Local-timezone text: SSR and the browser disagree
                              by design, so the mismatch is expected (w6/030). */}
                          <TableCell
                            className="whitespace-nowrap text-right text-muted-foreground"
                            suppressHydrationWarning
                          >
                            {formatDateTime(group.updatedAt) ?? "—"}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}
