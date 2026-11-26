import {
  Outlet,
  createFileRoute,
  useRouterState,
} from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageScroll } from "@/common/components/page-frame";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { CopyButton } from "@/common/components/copy-button";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import {
  resourceLoadErrorVariant,
  resourceNotFound,
  useNotFoundRedirect,
} from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";
import { SandboxGroupDocument } from "@/graphql/definitions";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import { useSandboxGroup } from "@/features/sandbox-groups/hooks/use-sandbox-groups";
import { sandboxGroupPageLabelKey } from "@/features/sandbox-group-pages/lib/nav";
import { SandboxGroupDetailProvider } from "@/features/sandbox-group-pages/lib/group-detail";
import {
  ImagesPageSkeleton,
  SandboxGroupSettingsSkeleton,
  SandboxesPageSkeleton,
  SnapshotsPageSkeleton,
} from "@/features/sandbox-group-pages/components/group-page-skeletons";

export const Route = createFileRoute("/sandbox-groups_/$sandboxGroupId")({
  staticData: { chrome: true },
  component: RouteComponent,
  // A dedicated pending frame at 0ms, as a Key Value instance's layout has:
  // the header and the open page's skeleton without the live <Outlet/>.
  pendingComponent: PendingRouteComponent,
  pendingMs: 0,
  codeSplitGroupings: [
    ["component", "pendingComponent"],
    ["errorComponent"],
    ["notFoundComponent"],
  ],
  beforeLoad: requireAuth(),
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: SandboxGroupDocument,
          variables: {
            id: params.sandboxGroupId,
            ownerId: context.workspaceId ?? undefined,
          },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) => (data?.sandboxGroup?.name?.trim() ? data.sandboxGroup : null),
    ),
  // Each page leads this with its own name (sandboxGroupPageHead).
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (group) => [
        group.name,
        translatedText("sandboxGroups.resourceType"),
      ]),
      match,
    ),
});

function PendingRouteComponent() {
  return <SandboxGroupLayout routePending />;
}

function RouteComponent() {
  useLoaderErrorRetry(Route.useLoaderData(), Route.useParams().sandboxGroupId);
  return <SandboxGroupLayout />;
}

/**
 * A sandbox group's pages share this layout (w1/m658), as Render's do: the
 * header (name, Default and id), the open page's heading, which is its
 * sidebar entry's label, and an `<Outlet/>` for the page. The sidebar is
 * DashboardSidebar's SandboxGroupSidebar: Sandboxes (the group's index),
 * Snapshots, Custom images and Settings.
 */
function SandboxGroupLayout({ routePending = false }: { routePending?: boolean }) {
  const { sandboxGroupId } = Route.useParams();
  const { t } = useTranslations();
  const { group: loaded, loading, error, refetch } = useSandboxGroup(sandboxGroupId);
  const adopting = useAdoptResourceWorkspace(
    sandboxGroupId,
    routePending ? null : loaded?.ownerId,
  );
  const group = adopting || routePending ? null : loaded;
  useNotFoundRedirect(!routePending && resourceNotFound(loaded, loading, error));
  const loadErrorVariant = routePending
    ? null
    : resourceLoadErrorVariant(loaded, loading, error);
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const pageLabelKey = sandboxGroupPageLabelKey(pathname, sandboxGroupId);

  return (
    <DashboardLayout>
      <div
        className="contents"
        data-route-skeleton={!group ? "sandbox-group-detail" : undefined}
      >
        <div
          data-skeleton-region={!group ? "resource-header" : undefined}
          className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b px-4 py-4 sm:px-6"
        >
          {group ? (
            <>
              <div className="flex min-w-0 max-w-full items-center gap-2">
                <h1 className="truncate text-xl font-semibold">{group.name}</h1>
                {group.isDefault ? (
                  <Badge variant="secondary">{t("sandboxGroups.defaultBadge")}</Badge>
                ) : null}
              </div>
              <span className="text-muted-foreground inline-flex min-w-0 items-center gap-1.5 font-mono text-xs">
                <span className="truncate">{group.id}</span>
                <CopyButton
                  value={group.id}
                  label={t("sandboxGroups.copyId")}
                  successText={t("sandboxGroups.copied")}
                  errorText={t("sandboxGroups.copyError")}
                />
              </span>
            </>
          ) : (
            <div
              data-skeleton-region="resource-title"
              className="flex min-w-0 max-w-full flex-wrap items-center gap-x-3 gap-y-1"
            >
              {/* The h1's text-xl line box is 28px; the id's line is the
                  copy button's 32px (t009: a 16px bar left the header 4px
                  short at 1440 and 16px at 390). */}
              <Skeleton className="h-7 w-48" />
              <div className="flex h-8 items-center">
                <Skeleton className="h-4 w-56" />
              </div>
            </div>
          )}
        </div>

        <PageScroll data-skeleton-region={!group ? "active-page" : undefined}>
          <div className="mx-auto w-full max-w-6xl space-y-6">
            {pageLabelKey ? (
              <h2 className="text-lg leading-7 font-semibold tracking-tight">
                {t(pageLabelKey)}
              </h2>
            ) : null}
            {loadErrorVariant ? (
              <ResourceLoadError
                onRetry={() => void refetch()}
                variant={loadErrorVariant}
              />
            ) : group ? (
              <SandboxGroupDetailProvider value={{ group, refetch: () => void refetch() }}>
                <Outlet />
              </SandboxGroupDetailProvider>
            ) : (
              <SandboxGroupPageSkeleton labelKey={pageLabelKey} />
            )}
          </div>
        </PageScroll>
      </div>
    </DashboardLayout>
  );
}

/** The open page's own pending frame, while the layout has no group yet. */
function SandboxGroupPageSkeleton({
  labelKey,
}: {
  labelKey: ReturnType<typeof sandboxGroupPageLabelKey>;
}) {
  switch (labelKey) {
    case "sandboxGroupPages.navSnapshots":
      return <SnapshotsPageSkeleton />;
    case "sandboxGroupPages.navImages":
      return <ImagesPageSkeleton />;
    case "sandboxGroupPages.navSettings":
      return <SandboxGroupSettingsSkeleton />;
    default:
      return <SandboxesPageSkeleton />;
  }
}
