import { useState } from "react";
import {
  Link,
  createFileRoute,
  useNavigate,
  useRouter,
} from "@tanstack/react-router";
import { ArrowLeft, AlertTriangle } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { EnvGroupActions } from "@/features/env-groups/components/env-group-actions";
import { EnvGroupEditors } from "@/features/env-groups/components/env-group-editors";
import { EnvGroupMetadata } from "@/features/env-groups/components/env-group-metadata";
import { LinkedServicesCard } from "@/features/env-groups/components/linked-services-card";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  classifyEnvGroupError,
  isEnvGroupNotFound,
  useEnvGroup,
  useEnvGroupMutations,
} from "@/features/env-groups/hooks/use-env-groups";
import { useServices } from "@/features/services/hooks/use-services";
import { useEnvGroupScopeIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { EnvGroupDocument } from "@/graphql/definitions";
import {
  isNotFoundError,
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

// The trailing underscore in this file route deliberately escapes the
// /env-groups list route's component hierarchy while preserving the public URL.
// The list is a page, not a layout, so nesting the detail beneath it would leave
// the list mounted because it has no <Outlet />.
export const Route = createFileRoute("/env-groups_/$groupId")({
  staticData: { chrome: true },
  component: EnvGroupDetailPage,
  // The page doubles as its own pending state at 0ms: it renders full
  // chrome + its skeleton stack while its Apollo read loads (tolerating the
  // absent loaderData), so the title-loader wait shows the real frame
  // instead of the router-level blank that used to flash white.
  pendingComponent: EnvGroupDetailPage,
  pendingMs: 0,
  beforeLoad: requireAuth(),
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: EnvGroupDocument,
          variables: { id: params.groupId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) => (data?.envGroup?.name?.trim() ? data.envGroup : null),
      isNotFoundError,
    ),
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (group) => [
        group.name,
        translatedText("envGroups.resourceType"),
      ]),
      match,
    ),
});

export function EnvGroupDetailPage() {
  const { groupId } = Route.useParams();
  const { t } = useTranslations();
  const navigate = useNavigate();
  const router = useRouter();
  const { group, loading, error, refetch } = useEnvGroup(groupId);
  const {
    services,
    loading: servicesLoading,
    error: servicesError,
  } = useServices();
  const scope = useEnvGroupScopeIndex();
  const [deleteInFlight, setDeleteInFlight] = useState(false);
  const mutations = useEnvGroupMutations(refetch, {
    skipRenameRefetch: true,
  });
  const renameGroup = async (id: string, name: string) => {
    const renamed = await mutations.renameGroup(id, name);
    if (renamed) void router.invalidate();
    return renamed;
  };
  const deleteGroup = async (id: string) => {
    // Deleting evicts this detail query before the action callback navigates
    // back to the collection. Suppress the generic dead-id redirect during
    // that intentional gap so it cannot race the collection navigation.
    setDeleteInFlight(true);
    const deleted = await mutations.deleteGroup(id);
    if (!deleted) setDeleteInFlight(false);
    return deleted;
  };
  const errorKind = classifyEnvGroupError(error);
  const notFound = isEnvGroupNotFound(error);
  const environmentLabel = group?.environmentId
    ? (scope.byId.get(group.environmentId)?.name ??
      t("envGroups.unknownEnvironment", { id: group.environmentId }))
    : t("envGroups.workspaceScope");

  // A dead id — a not-found-shaped error, or a null row with no error at all —
  // redirects home (w9/m55); any other failed query stays put on the inline
  // error state so an outage never masquerades as a deleted group. A
  // roll-window loader failure re-runs once (w1/m52) so the title recovers.
  useNotFoundRedirect(
    !deleteInFlight && !loading && !group && (notFound || !errorKind),
  );
  useLoaderErrorRetry(Route.useLoaderData(), groupId);

  return (
    <DashboardLayout>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6">
        <div className="flex min-w-0 items-center gap-3">
          <Button variant="ghost" size="icon" asChild>
            <Link to="/env-groups" aria-label={t("envGroups.backToList")}>
              <ArrowLeft />
            </Link>
          </Button>
          <div className="min-w-0">
            <h1 className="truncate text-xl font-semibold">
              {group?.name ?? groupId}
            </h1>
            <p className="truncate font-mono text-xs text-muted-foreground">
              {groupId}
            </p>
          </div>
        </div>
        {group ? (
          <EnvGroupActions
            group={group}
            environments={scope.environments}
            renameGroup={renameGroup}
            moveGroup={mutations.moveGroup}
            cloneGroup={mutations.cloneGroup}
            deleteGroup={deleteGroup}
            busy={mutations.busy}
            onDeleted={() =>
              void navigate({ to: "/env-groups", replace: true })
            }
            onCloned={(cloneId) =>
              void navigate({
                to: "/env-groups/$groupId",
                params: { groupId: cloneId },
              })
            }
          />
        ) : null}
      </div>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-5xl space-y-6">
          {(loading || notFound || !errorKind) && !group ? (
            <>
              <Skeleton className="h-64" />
              <Skeleton className="h-64" />
            </>
          ) : errorKind && !group ? (
            <div className="flex flex-col items-center gap-2 py-12 text-center">
              <AlertTriangle className="size-8 text-destructive" />
              <p className="font-medium">{t(`envGroups.${errorKind}Title`)}</p>
              <p className="text-sm text-muted-foreground">
                {t(`envGroups.${errorKind}Body`)}
              </p>
            </div>
          ) : !group ? null : (
            <>
              <Card>
                <CardHeader>
                  <CardTitle>{t("envGroups.metadataTitle")}</CardTitle>
                </CardHeader>
                <CardContent>
                  <EnvGroupMetadata
                    group={group}
                    environmentLabel={environmentLabel}
                  />
                </CardContent>
              </Card>
              <EnvGroupEditors
                group={group}
                loading={loading}
                error={error}
                refetch={refetch}
              />
              <LinkedServicesCard
                group={group}
                services={services}
                linkGroup={mutations.linkGroup}
                unlinkGroup={mutations.unlinkGroup}
                busy={mutations.busy || servicesLoading || scope.loading}
                error={servicesError ?? scope.error}
                serviceEnvironmentById={scope.serviceEnvironmentById}
                scopeReady={!scope.loading && !scope.error}
              />
            </>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}
