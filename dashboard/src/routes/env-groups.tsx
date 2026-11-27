import { useState } from "react";
import { EnvGroupsTable } from "@/features/env-groups/components/env-groups-table";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  EnvGroupsContentSkeleton,
  EnvGroupsListPageSkeleton,
} from "@/common/components/route-skeletons";
import { AlertTriangle, Layers3, Plus, Search, X } from "lucide-react";
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
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { NewEnvGroupDialog } from "@/features/env-groups/components/new-env-group-dialog";
import { useEnvGroups } from "@/features/env-groups/hooks/use-env-groups";
import { classifySecretStoreError } from "@/features/services/lib/secret-store";
import { useServices } from "@/features/services/hooks/use-services";
import { useEnvGroupScopeIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export const Route = createFileRoute("/env-groups")({
  staticData: { chrome: true },
  component: EnvGroupsPage,
  pendingComponent: EnvGroupsListPageSkeleton,
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
  // `?new=env-group` opens New Environment Group: the URL owns the dialog, so
  // Render's `/new/env-group` lands in it (w3/m705; the `?new=` idiom of
  // routes/index.tsx).
  validateSearch: (search: Record<string, unknown>): { new?: "env-group" } => ({
    new: search.new === "env-group" ? "env-group" : undefined,
  }),
  head: ({ match }) => translatedTitleHead("envGroups.pageTitle", match),
});

export function EnvGroupsPage() {
  const hydrated = useIsHydrated();
  const { t } = useTranslations();
  // The header button and the empty state both open the one create dialog,
  // whose open state the URL owns (`?new=env-group`); `replace` keeps opening
  // and closing it out of the back-button history.
  const { new: newParam } = Route.useSearch();
  const routeNavigate = Route.useNavigate();
  const createOpen = newParam === "env-group";
  const setCreateOpen = (open: boolean) =>
    void routeNavigate({
      search: open ? { new: "env-group" } : {},
      replace: true,
    });
  const { currentWorkspaceId } = useWorkspace();
  const navigate = useNavigate();
  const { groups, loading, error, refetch } = useEnvGroups();
  const {
    services,
    loading: servicesLoading,
    error: servicesError,
    refetch: refetchServices,
  } = useServices();
  const scope = useEnvGroupScopeIndex();
  const [searchState, setSearchState] = useState({
    workspaceId: currentWorkspaceId,
    value: "",
  });
  const search =
    searchState.workspaceId === currentWorkspaceId ? searchState.value : "";
  const setSearch = (value: string) =>
    setSearchState({ workspaceId: currentWorkspaceId, value });
  const errorKind = classifySecretStoreError(error);
  const initialLoading = loading && groups.length === 0;
  const normalizedSearch = search.trim().toLocaleLowerCase();
  const visibleGroups = normalizedSearch
    ? groups.filter((group) =>
        group.name.toLocaleLowerCase().includes(normalizedSearch),
      )
    : groups;

  return (
    <DashboardLayout>
      <PageFrame
        header={
          <PageHeader
            title={t("envGroups.pageTitle")}
            description={t("envGroups.pageDescription")}
            actions={
              <Button size="sm" onClick={() => setCreateOpen(true)}>
                <Plus />
                {t("envGroups.newButton")}
              </Button>
            }
          />
        }
      >
        <NewEnvGroupDialog
          key={currentWorkspaceId}
          open={createOpen}
          onOpenChange={setCreateOpen}
          refetch={refetch}
          services={services}
          servicesLoading={servicesLoading}
          servicesError={servicesError}
          environments={scope.environments}
          projects={scope.projects}
          serviceEnvironmentById={scope.serviceEnvironmentById}
          scopeLoading={scope.loading}
          scopeReady={scope.ready}
          scopeError={scope.error}
          onRetry={() => {
            scope.retry();
            void refetchServices().catch(() => undefined);
          }}
          onCreated={(groupId) =>
            void navigate({
              to: "/env-groups/$groupId",
              params: { groupId },
            })
          }
        />
        {errorKind && groups.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-12 text-center">
            <AlertTriangle className="size-8 text-destructive" />
            <p className="font-medium">{t(`envGroups.${errorKind}Title`)}</p>
            <p className="text-sm text-muted-foreground">
              {t(`envGroups.${errorKind}Body`)}
            </p>
          </div>
        ) : initialLoading ? (
          <EnvGroupsContentSkeleton />
        ) : groups.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-12 text-center">
            <Layers3 className="size-8 text-muted-foreground" />
            <p className="font-medium">{t("envGroups.emptyTitle")}</p>
            <p className="max-w-md text-sm text-muted-foreground">
              {t("envGroups.emptyBody")}
            </p>
            <Button
              size="sm"
              className="mt-2"
              onClick={() => setCreateOpen(true)}
            >
              <Plus />
              {t("envGroups.newButton")}
            </Button>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <div className="relative max-w-md flex-1">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={search}
                  // Uneditable until handlers attach: a query typed into
                  // the server HTML is wiped by hydration (w4/m219).
                  disabled={!hydrated}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={t("envGroups.searchPlaceholder")}
                  aria-label={t("envGroups.searchLabel")}
                  className="pl-9"
                />
              </div>
              {search ? (
                <Button variant="ghost" size="sm" onClick={() => setSearch("")}>
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
                <EnvGroupsTable groups={visibleGroups} scope={scope} />
              </div>
            )}
          </div>
        )}
      </PageFrame>
    </DashboardLayout>
  );
}
