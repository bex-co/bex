import { Link, createFileRoute } from "@tanstack/react-router";
import { AlertTriangle, ArrowLeft } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { CopyButton } from "@/common/components/copy-button";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
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
import { SandboxGroupCredentialsPanel } from "@/features/sandbox-groups/components/sandbox-group-credentials-panel";
import { SandboxGroupDetailRouteSkeleton } from "@/features/sandbox-groups/components/sandbox-group-skeletons";
import { useSandboxGroup } from "@/features/sandbox-groups/hooks/use-sandbox-groups";

export const Route = createFileRoute("/sandbox-groups_/$sandboxGroupId")({
  staticData: { chrome: true },
  component: SandboxGroupDetailPage,
  pendingComponent: SandboxGroupDetailRouteSkeleton,
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
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (group) => [
        group.name,
        translatedText("sandboxGroups.resourceType"),
      ]),
      match,
    ),
});

function SandboxGroupDetailPage() {
  const { sandboxGroupId } = Route.useParams();
  const { t } = useTranslations();
  const { group: loaded, loading, error, notFound, refetch } =
    useSandboxGroup(sandboxGroupId);
  const adopting = useAdoptResourceWorkspace(sandboxGroupId, loaded?.ownerId);
  const group = adopting ? null : loaded;

  useNotFoundRedirect(!loading && !loaded && (notFound || !error));
  useLoaderErrorRetry(Route.useLoaderData(), sandboxGroupId);

  if (loading || adopting) {
    return <SandboxGroupDetailRouteSkeleton />;
  }

  if (!group) {
    return (
      <DashboardLayout>
        <PageFrame
          header={
            <PageHeader
              leading={
                <Button asChild size="icon-sm" variant="ghost">
                  <Link
                    to="/sandbox-groups"
                    aria-label={t("sandboxGroups.backToList")}
                  >
                    <ArrowLeft className="size-4" aria-hidden="true" />
                  </Link>
                </Button>
              }
              title={t("sandboxGroups.resourceType")}
            />
          }
        >
          <div className="flex flex-col items-center gap-2 py-12 text-center">
            <AlertTriangle
              className="text-muted-foreground size-6"
              aria-hidden="true"
            />
            <p className="text-muted-foreground text-sm">
              {t("sandboxGroups.detailLoadError")}
            </p>
            <Button size="sm" variant="outline" onClick={() => void refetch()}>
              {t("common.tryAgain")}
            </Button>
          </div>
        </PageFrame>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      <PageFrame
        header={
          <PageHeader
            leading={
              <Button asChild size="icon-sm" variant="ghost">
                <Link
                  to="/sandbox-groups"
                  aria-label={t("sandboxGroups.backToList")}
                >
                  <ArrowLeft className="size-4" aria-hidden="true" />
                </Link>
              </Button>
            }
            title={group.name}
            titleAddon={
              group.isDefault ? (
                <Badge variant="secondary">
                  {t("sandboxGroups.defaultBadge")}
                </Badge>
              ) : null
            }
            description={
              <span className="inline-flex items-center gap-1.5 font-mono text-xs">
                <span>{group.id}</span>
                <CopyButton
                  value={group.id}
                  label={t("sandboxGroups.copyId")}
                  successText={t("sandboxGroups.copied")}
                  errorText={t("sandboxGroups.copyError")}
                />
              </span>
            }
          />
        }
      >
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <Card className="gap-1.5 py-4">
            <CardHeader className="px-4 py-0">
              <CardDescription>{t("sandboxGroups.fieldRegion")}</CardDescription>
              <CardTitle className="text-base font-semibold">
                {group.region}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card className="gap-1.5 py-4">
            <CardHeader className="px-4 py-0">
              <CardDescription>
                {t("sandboxGroups.fieldConcurrencyLimit")}
              </CardDescription>
              <CardTitle className="text-base font-semibold tabular-nums">
                {group.concurrencyLimit}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card className="gap-1.5 py-4">
            <CardHeader className="px-4 py-0">
              <CardDescription>
                {t("sandboxGroups.fieldDefault")}
              </CardDescription>
              <CardTitle className="text-base font-semibold">
                {group.isDefault
                  ? t("sandboxGroups.defaultYes")
                  : t("sandboxGroups.defaultNo")}
              </CardTitle>
            </CardHeader>
          </Card>
        </div>

        <SandboxGroupCredentialsPanel
          sandboxGroupId={group.id}
          ownerId={group.ownerId}
        />
      </PageFrame>
    </DashboardLayout>
  );
}
